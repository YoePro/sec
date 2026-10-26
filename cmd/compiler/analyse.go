package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// analyseReportClass is the §61(3) presentation class of one reported analysis
// result. Classes are presentation only; the canonical structured facts stay
// owned by the producing analysis.
type analyseReportClass string

const (
	analyseClassError    analyseReportClass = "error"
	analyseClassUnproven analyseReportClass = "unproven"
	analyseClassWarning  analyseReportClass = "warning"
	analyseClassAdvisory analyseReportClass = "advisory"
)

// analyseCommand is the parsed `sec analyse` invocation. Every available
// analysis runs by default; `--all` is the explicit spelling of that default.
//
// Rules:
//   - rules/tooling/cli.md — § 5 (`sec analyse`)
//   - rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — §§ 2–7
type analyseCommand struct {
	inputs       []string
	target       CompilerTarget
	all          bool
	family       string
	function     string
	depth        sema.AnalysisDepth
	presentation analysePresentation
}

// parseAnalyseCommandArgs accepts
// `sec analyse [--all | <analysis>] [-f|--function <callable>]
// [-d|--depth interactive|standard|deep] [--info|--verbose|--json]
// [--target <os-arch>] [<inputs>...]`. Without inputs the current directory
// is analysed. An unknown analysis name or option never falls back to
// `--all`.
func parseAnalyseCommandArgs(args []string, host CompilerTarget) (analyseCommand, error) {
	command := analyseCommand{target: host, depth: sema.AnalysisDeep, presentation: analysePresentDefault}
	presentations := map[analysePresentation]bool{}
	value := func(index *int, arg, name string) (string, error) {
		if prefix := name + "="; strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix), nil
		}
		if *index+1 >= len(args) {
			return "", fmt.Errorf("%s requires a value", name)
		}
		*index++
		return args[*index], nil
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--all":
			command.all = true
		case arg == "--target" || strings.HasPrefix(arg, "--target="):
			text, err := value(&index, arg, "--target")
			if err != nil {
				return analyseCommand{}, err
			}
			target, ok := parseCompilerTarget(text)
			if !ok {
				return analyseCommand{}, fmt.Errorf("invalid target %q", text)
			}
			command.target = target
		case arg == "-f" || arg == "--function" || strings.HasPrefix(arg, "--function="):
			name := "--function"
			if arg == "-f" {
				name = "-f"
			}
			text, err := value(&index, arg, name)
			if err != nil {
				return analyseCommand{}, err
			}
			if command.function != "" {
				return analyseCommand{}, fmt.Errorf("only one --function root may be selected")
			}
			command.function = text
		case arg == "-d" || arg == "--depth" || strings.HasPrefix(arg, "--depth="):
			name := "--depth"
			if arg == "-d" {
				name = "-d"
			}
			text, err := value(&index, arg, name)
			if err != nil {
				return analyseCommand{}, err
			}
			depth, err := sema.ParseAnalysisDepth(text)
			if err != nil {
				return analyseCommand{}, fmt.Errorf("%s must be interactive, standard or deep, got %q", name, text)
			}
			command.depth = depth
		case arg == "--info":
			presentations[analysePresentInfo] = true
		case arg == "--verbose":
			presentations[analysePresentVerbose] = true
		case arg == "--json":
			presentations[analysePresentJSON] = true
		case strings.HasPrefix(arg, "-"):
			options := []string{"--all", "--function", "-f", "--depth", "-d", "--info", "--verbose", "--json", "--target"}
			return analyseCommand{}, fmt.Errorf("unknown analyse option %s%s", arg, nearestSuggestion(arg, options))
		case command.family == "" && len(command.inputs) == 0 && looksLikeAnalysisName(arg):
			if _, known := findAnalysisFamily(arg); !known {
				return analyseCommand{}, fmt.Errorf("unknown analysis %q%s; available analyses: %s", arg, nearestSuggestion(arg, analysisFamilyNames()), strings.Join(analysisFamilyNames(), ", "))
			}
			command.family = arg
		default:
			command.inputs = append(command.inputs, arg)
		}
	}
	if command.all && command.family != "" {
		return analyseCommand{}, fmt.Errorf("--all and the analysis name %q select different analyses; use one", command.family)
	}
	switch {
	case presentations[analysePresentJSON] && (presentations[analysePresentInfo] || presentations[analysePresentVerbose]):
		return analyseCommand{}, fmt.Errorf("--json is a separate output format; it cannot be combined with --info or --verbose")
	case presentations[analysePresentJSON]:
		command.presentation = analysePresentJSON
	case presentations[analysePresentVerbose]:
		// --verbose subsumes --info.
		command.presentation = analysePresentVerbose
	case presentations[analysePresentInfo]:
		command.presentation = analysePresentInfo
	}
	if len(command.inputs) == 0 {
		command.inputs = []string{"."}
	}
	return command, nil
}

// runAnalyseCommand performs analysis-only compilation: the same source, plan
// and Sema facts as build/check at the requested depth (Deep by default),
// without Semantic IR or code generation, and presents the requested
// analyses for the whole program or one callable's reachable call graph.
//
// Rules:
//   - rules/compiler/compiler_pipeline.md — § 65(1–3) analysis-only mode
//   - rules/compiler/compiler_analysis.md — § 61 `sec analyse`; § 62 analysis modes
//   - rules/tooling/cli.md — § 5 `sec analyse`
func runAnalyseCommand(args []string, out io.Writer) {
	command, err := parseAnalyseCommandArgs(args, hostCompilerTarget())
	if err != nil {
		reportToolError("usage", "%v", err)
		fmt.Fprintln(os.Stderr, "run `sec help analyse` for the analysis names, options and examples")
		exitCLI(1)
	}
	if command.family != "" {
		if family, _ := findAnalysisFamily(command.family); family.Availability != analysisAvailable {
			reportToolError("analysis", "the %s analysis cannot run on its own: %s", family.Name, family.Limitation)
			exitCLI(1)
		}
	}
	var report analyseReport
	if command.presentation == analysePresentJSON {
		cliDiagnostics.format = diagnosticFormatJSON
		cliDiagnostics.documentWriter = func(occurrences []emittedOccurrence, summary diagnosticSummary) {
			writeAnalysisJSON(out, report.document(command, occurrences, summary))
		}
	}
	deferSemaWarnings = true
	program, summary := parseSourceInputs(command.inputs, command.target, true)
	selected := analyseSelectedFiles(program)
	analyzer := analyzeProgramWithSourcesAtDepth(program, command.target, collectSourceRootsFromInputs(command.inputs), summary, false, command.depth)
	var scope *analyseScope
	if command.function != "" {
		scope, err = resolveAnalyseScope(analyzer.CallGraph(), command.function, selected)
		if err != nil {
			reportToolError("analysis", "%v", err)
			exitCLI(1)
		}
	}
	report = buildAnalyseReport(analyzer, selected, command.target, command.family, scope)
	report.completed = true
	if command.presentation != analysePresentJSON {
		report.writeHuman(out, command)
	}
	if report.counts[analyseClassError] > 0 {
		exitCLI(3)
	}
}

// analyseSelectedFiles records the user-selected source files before trusted
// core and stdlib sources are merged, so reports cover only requested code.
func analyseSelectedFiles(program *ast.Program) map[string]bool {
	files := map[string]bool{}
	for _, stmt := range program.Statements {
		if file := statementTokenForSource(stmt).File; file != "" {
			files[file] = true
		}
	}
	return files
}

type analyseReport struct {
	target   CompilerTarget
	depth    sema.AnalysisDepth
	sections []analyseSection
	counts   map[analyseReportClass]int
	// family is the selected analysis ("" for all), scope the function focus.
	family string
	scope  *analyseScope
	names  map[sema.CallableID]string
	// warnings are the selected registered and unregistered diagnostics;
	// findings the reportable analysis results; limitations what kept the
	// result from being complete.
	warnings    []sema.Error
	findings    []analyseFinding
	limitations []string
	completed   bool
}

type analyseSection struct {
	title string
	lines []string
}

func (r *analyseReport) section(title string) *analyseSection {
	r.sections = append(r.sections, analyseSection{title: title})
	return &r.sections[len(r.sections)-1]
}

func (s *analyseSection) add(format string, args ...any) {
	s.lines = append(s.lines, fmt.Sprintf(format, args...))
}

// buildAnalyseReport presents the structured facts the compiler already
// produced. It never re-derives a fact, so compiler, LSP and `sec analyse`
// stay on one fact model.
//
// Rules:
//   - rules/compiler/compiler.md — § 56(4) one shared fact model
//   - rules/compiler/compiler_analysis.md — § 61(2) report contents; § 61(3) report classes
func buildAnalyseReport(analyzer *sema.Analyzer, selected map[string]bool, target CompilerTarget, family string, scope *analyseScope) analyseReport {
	report := analyseReport{target: target, depth: analyzer.AnalysisDepth(), counts: map[analyseReportClass]int{}, family: family, scope: scope}
	inSelection := func(token lexer.Token) bool {
		return selected[token.File] && (scope == nil || scope.includes(token))
	}
	for _, warning := range analyzer.Warnings() {
		token := lexer.Token{File: warning.File, Line: warning.Line, Column: warning.Column}
		if !inSelection(token) || family != "" && analyseFamilyOfWarning(warning) != family {
			continue
		}
		report.warnings = append(report.warnings, warning)
		report.counts[analyseDiagnosticClass(warning)]++
	}
	graph := analyzer.CallGraph()
	names := map[sema.CallableID]string{}
	for _, node := range graph.Nodes() {
		names[node.ID] = analyseCallableName(node)
	}
	report.names = names
	nodes := []sema.CallableNode{}
	for _, node := range graph.Nodes() {
		if inSelection(node.Declaration) {
			nodes = append(nodes, node)
		}
	}
	if family == "" {
		report.addCallGraph(graph, nodes, names, inSelection)
		report.addIterators(analyzer, inSelection)
		report.addEffects(graph, nodes, names)
		report.addAllocationFacts(analyzer, nodes, names)
		report.addEscape(analyzer.EscapeAnalysis(), inSelection)
	}
	if family == "" || family == "parameter-usage" {
		report.addParameterUsage(analyzer.ParameterUsageAnalysis(), inSelection)
	}
	if family == "" || family == "pitfalls" {
		report.addPitfalls(analyzer.PitfallAnalysis(), inSelection)
	}
	if family == "" || family == "closures" {
		report.addClosures(analyzer, inSelection)
	}
	if report.depth != sema.AnalysisDeep {
		report.limitations = append(report.limitations, fmt.Sprintf("depth %s: advisory refinement may be reduced; anything not proven is reported as unproven, never as valid", report.depth))
	}
	sort.SliceStable(report.findings, func(i, j int) bool {
		left, right := report.findings[i].Token, report.findings[j].Token
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		return report.findings[i].Rule < report.findings[j].Rule
	})
	return report
}

// finding records one reportable result with the callable that owns it in a
// focused analysis.
func (r *analyseReport) finding(finding analyseFinding) {
	if r.scope != nil {
		if owner, ok := r.scope.owner(finding.Token); ok {
			finding.Callable = owner
		}
	}
	r.findings = append(r.findings, finding)
}

// limitation records an incompleteness that must stay visible even in the
// default presentation (rules/tooling/cli.md § 5.4(7)).
func (r *analyseReport) limitation(format string, args ...any) {
	r.limitations = append(r.limitations, fmt.Sprintf(format, args...))
}

func analyseCallableName(node sema.CallableNode) string {
	if node.ImplTarget != "" && !strings.HasPrefix(node.Name, node.ImplTarget+".") {
		return node.ImplTarget + "." + node.Name
	}
	return node.Name
}

func analysePosition(token lexer.Token) string {
	return fmt.Sprintf("%s:%d:%d", token.File, token.Line, token.Column)
}

func analysePath(path []sema.CallableID, names map[sema.CallableID]string) string {
	parts := make([]string, len(path))
	for index, id := range path {
		parts[index] = names[id]
		if parts[index] == "" {
			parts[index] = string(id)
		}
	}
	return strings.Join(parts, " -> ")
}

// addCallGraph reports roots and resolved call sites of the selected callables.
// Rule: rules/analysis/call_graph.md — call graph is a compiler-owned fact.
func (r *analyseReport) addCallGraph(graph *sema.CallGraph, nodes []sema.CallableNode, names map[sema.CallableID]string, inSelection func(lexer.Token) bool) {
	section := r.section("call graph")
	sites := 0
	for _, root := range graph.Roots() {
		if inSelection(root.Source) {
			section.add("root %s %s", root.Kind, names[root.Node])
		}
	}
	for _, node := range nodes {
		for _, site := range graph.Outgoing(node.ID) {
			sites++
			section.add("%s -> %s [%s, %s] at %s", names[node.ID], analysePath(site.Targets, names), site.Dispatch, site.Execution, analysePosition(site.Source))
		}
	}
	section.lines = append([]string{fmt.Sprintf("%d callables, %d call sites", len(nodes), sites)}, section.lines...)
}

// addEffects reports direct effects and the canonical shortest panic and
// allocation cause paths.
// Rule: rules/analysis/effect_analysis.md — effect summaries and cause paths.
func (r *analyseReport) addEffects(graph *sema.CallGraph, nodes []sema.CallableNode, names map[sema.CallableID]string) {
	section := r.section("effects")
	for _, node := range nodes {
		effects := graph.EffectSummary(node.ID)
		for _, effect := range effects.DirectEffects {
			section.add("%s: %s at %s", names[node.ID], effect.Kind, analysePosition(effect.Source))
		}
		if effects.MayPanic && len(effects.PanicPath) > 1 {
			section.add("%s: may panic via %s", names[node.ID], analysePath(effects.PanicPath, names))
		}
		addPanicCauseReport(section, effects.PanicCauses, names[node.ID], names)
		arena := graph.ArenaSummary(node.ID)
		for _, effect := range arena.DirectEffects {
			section.add("%s: arena %s %s at %s", names[node.ID], effect.Kind, effect.Arena, analysePosition(effect.Source))
		}
		if arena.MayAllocate && len(arena.AllocationPath) > 1 {
			section.add("%s: may allocate via %s", names[node.ID], analysePath(arena.AllocationPath, names))
		}
	}
}

// addEscape reports non-trivial parameter dispositions.
// Rule: rules/analysis/escape_analysis.md — callable escape summaries.
func (r *analyseReport) addEscape(escape *sema.EscapeAnalysis, inSelection func(lexer.Token) bool) {
	section := r.section("escape")
	for _, summary := range escape.Summaries() {
		if !inSelection(summary.Declaration) {
			continue
		}
		for _, parameter := range summary.Parameters {
			if dispositions := analyseEscapeDispositions(parameter.Dispositions); dispositions != "" {
				section.add("%s(%s): %s", summary.Name, parameter.Name, dispositions)
			}
		}
		if dispositions := analyseEscapeDispositions(summary.Receiver); dispositions != "" {
			section.add("%s(receiver): %s", summary.Name, dispositions)
		}
		if summary.Unknown {
			section.add("%s: unknown escape behavior", summary.Name)
		}
	}
}

func analyseEscapeDispositions(dispositions []sema.EscapeParameterDisposition) string {
	parts := []string{}
	for _, disposition := range dispositions {
		if disposition != sema.EscapeParameterNoEscape {
			parts = append(parts, string(disposition))
		}
	}
	return strings.Join(parts, ", ")
}

// addParameterUsage reports semantic demand and candidate confidence, reasons,
// and blockers, keeping estimated copy cost separate from capability proof.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — `sec analyse`; Deep output (declared capability, required demand)
//   - rules/analysis/parameter_usage_analysis.md — "Recommendation confidence", "Recommendation reasons", "Blocked narrowing advisory"
func (r *analyseReport) addParameterUsage(usage *sema.ParameterUsageAnalysis, inSelection func(lexer.Token) bool) {
	section := r.section("parameter usage")
	if iterations, converged := usage.InterproceduralStatus(); !converged {
		section.add("incomplete demand refinement after %d iteration(s); unresolved call demand is conservative", iterations)
		r.limitation("parameter usage: incomplete demand refinement after %d iteration(s)", iterations)
	}
	if coverage := usage.ImportCoverage(); coverage.SkippedCallables > 0 {
		section.add("incomplete imported signature coverage: %d callable(s) skipped; work %d/%d, type depth limit %d", coverage.SkippedCallables, coverage.VisitedWork, coverage.MaxWork, coverage.MaxTypeDepth)
		r.limitation("parameter usage: %d imported callable(s) skipped", coverage.SkippedCallables)
	}
	for _, summary := range usage.Summaries() {
		if !inSelection(summary.Declaration) {
			continue
		}
		parameters := append([]sema.ParameterUsageParameterSummary(nil), summary.Parameters...)
		if summary.Receiver != nil {
			parameters = append([]sema.ParameterUsageParameterSummary{*summary.Receiver}, parameters...)
		}
		for _, parameter := range parameters {
			demand := parameter.Demand
			shapes := make([]string, len(demand.Shapes))
			for index, shape := range demand.Shapes {
				shapes[index] = string(shape)
			}
			section.add("%s(%s %s): access %s, mutation %s, ownership %s, lifetime %s, identity %s, shape [%s], precision %s",
				summary.Name, parameter.Name, sema.TypeDisplayName(parameter.DeclaredType), demand.Access, demand.Mutation, demand.Ownership, demand.Lifetime, demand.Identity, strings.Join(shapes, ", "), demand.Precision)
		}
	}
	recommendations, coverage := usage.RecommendationsWithCoverage()
	if coverage.SkippedCandidates > 0 {
		section.add("incomplete recommendation coverage: %d candidate(s) skipped; work %d/%d, type depth limit %d", coverage.SkippedCandidates, coverage.VisitedWork, coverage.MaxWork, coverage.MaxTypeDepth)
		r.limitation("parameter usage: %d recommendation candidate(s) skipped", coverage.SkippedCandidates)
	}
	for _, recommendation := range recommendations {
		if !inSelection(recommendation.Source) {
			continue
		}
		// Recommended narrowings are already reported as the registered
		// performance.large-value-parameter diagnostic; they are not a second
		// finding (compiler_analysis.md § 61(3)).
		section.add("%s(%s): candidate %s, %s", recommendation.CallableName, recommendation.Parameter, recommendation.Candidate, recommendation.Status)
		if recommendation.Confidence != "" {
			section.add("  confidence %s", recommendation.Confidence)
		}
		for _, reason := range recommendation.Reasons {
			section.add("  reason: %s", reason)
		}
		for _, blocker := range recommendation.Blockers {
			section.add("  blocked: %s", blocker)
		}
		if recommendation.SizeKnown {
			section.add("  estimated size: %d bytes", recommendation.EstimatedSizeBytes)
		}
	}
}

// pitfallReportClass keeps proven invalidity an error under its owning rule
// and presents intent inference as advisory insight.
// Rule: rules/analysis/pitfall_analysis.md — "Proven invalidity is not a warning"; Classification.
func pitfallReportClass(finding sema.PitfallFinding) analyseReportClass {
	if finding.Classification == sema.PitfallProvenInvalid {
		return analyseClassError
	}
	return analyseClassAdvisory
}

// addPitfalls reports findings, suppressed findings with reasons, proven
// fixes versus suggested edits, preferred canonical idioms and replacement text,
// and rules that were not evaluated.
// Rules: rules/analysis/pitfall_analysis.md — sec analyse Deep output,
// "Canonical idiom guidance", "Fix safety".
func (r *analyseReport) addPitfalls(pitfalls *sema.PitfallAnalysis, inSelection func(lexer.Token) bool) {
	section := r.section("pitfalls")
	if coverage := pitfalls.Coverage(); coverage.SkippedUnits > 0 {
		section.add("incomplete coverage: %d unit(s) skipped; syntax search %d/%d nodes, depth limit %d", coverage.SkippedUnits, coverage.VisitedNodes, coverage.MaxNodes, coverage.MaxDepth)
		r.limitation("pitfalls: %d unit(s) not analysed within the budget", coverage.SkippedUnits)
	}
	for _, finding := range pitfalls.Results() {
		if !inSelection(finding.Subject.Source) {
			continue
		}
		position := analysePosition(finding.Subject.Source)
		if finding.State == sema.PitfallStateSuppressed {
			reason := ""
			if finding.Suppression != nil {
				reason = ": " + finding.Suppression.Reason
			}
			section.add("suppressed %s at %s%s", finding.Rule, position, reason)
			continue
		}
		if finding.DiagnosticID != "" {
			section.add("owned by %s: %s at %s (supporting semantic evidence; not a second diagnostic)", finding.DiagnosticID, finding.Rule, position)
			continue
		}
		class := pitfallReportClass(finding)
		r.counts[class]++
		actions := []string{}
		for _, action := range finding.Actions {
			actions = append(actions, fmt.Sprintf("%s: %s", action.Kind, action.Title))
		}
		r.finding(analyseFinding{Family: "pitfalls", Class: class, Rule: string(finding.Rule), Token: finding.Subject.Source, Message: finding.Subject.Expression, Actions: actions})
		section.add("%s %s at %s: %s (%s, %s, owning rule %s)", class, finding.Rule, position, finding.Subject.Expression, finding.Classification, finding.Confidence, finding.OwningRule)
		for _, evidence := range finding.EvidenceFor {
			section.add("  evidence %s: %s", evidence.Strength, evidence.Fact)
		}
		for _, evidence := range finding.EvidenceAgainst {
			section.add("  evidence against %s: %s", evidence.Strength, evidence.Fact)
		}
		for _, action := range finding.Actions {
			section.add("  %s: %s", action.Kind, action.Title)
			if action.Idiom != "" {
				section.add("    canonical idiom: %s", action.Idiom)
			}
			if action.Replacement != "" {
				section.add("    replacement: %s", action.Replacement)
			}
		}
	}
	notEvaluated := []string{}
	for _, evaluation := range pitfalls.Evaluations() {
		if evaluation.Incomplete {
			notEvaluated = append(notEvaluated, fmt.Sprintf("incomplete %s (%s)", evaluation.Rule, evaluation.State))
		} else if evaluation.State == sema.PitfallStateNotEvaluated || evaluation.State == sema.PitfallStatePending {
			notEvaluated = append(notEvaluated, fmt.Sprintf("not evaluated %s (%s)", evaluation.Rule, evaluation.State))
		}
	}
	sort.Strings(notEvaluated)
	for _, rule := range notEvaluated {
		section.add("%s", rule)
	}
}

// write renders the recorded snapshot metadata and findings without assigning
// a synthetic analysis depth to a report assembled without snapshot metadata.
// Rules: rules/compiler/compiler_analysis.md — §61(2–3), §62(1–3).
func (r analyseReport) write(out io.Writer) {
	depth := string(r.depth)
	if depth == "" {
		depth = "unavailable"
	}
	fmt.Fprintf(out, "analysis report (depth %s, target %s)\n", depth, r.target.String())
	for _, section := range r.sections {
		fmt.Fprintf(out, "\n%s:\n", section.title)
		if len(section.lines) == 0 {
			fmt.Fprintln(out, "  none")
		}
		for _, line := range section.lines {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
	fmt.Fprintf(out, "\nresults: %d errors, %d unproven, %d warnings, %d advisory\n",
		r.counts[analyseClassError], r.counts[analyseClassUnproven], r.counts[analyseClassWarning], r.counts[analyseClassAdvisory])
}

// writeHuman renders the default, --info and --verbose presentations. The
// default shows only what needs attention — diagnostics, findings and
// limitations — and prints nothing for a clean, complete result; --info adds
// a short summary; --verbose adds every section of facts (and the summary).
//
// Rules:
//   - rules/tooling/cli.md — § 5.4 (human-readable output)
//   - rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — § 5
func (r analyseReport) writeHuman(out io.Writer, command analyseCommand) {
	for _, warning := range r.warnings {
		label := "Warning"
		if warning.Severity == diagnostics.SeverityInformation {
			label = "Info"
		}
		if warning.Severity == "" {
			warning.Severity = diagnostics.SeverityWarning
		}
		cliDiagnostics.semaDiagnostic(warning, fmt.Sprintf("%s: %s\n", label, warning))
	}
	if command.presentation == analysePresentVerbose {
		r.write(out)
		r.writeSummary(out, false)
		return
	}
	for _, finding := range r.findings {
		fmt.Fprintf(out, "%s %s at %s: %s\n", finding.Class, finding.Rule, analysePosition(finding.Token), finding.Message)
		if r.scope != nil && finding.Callable != "" && finding.Callable != r.scope.root.ID {
			fmt.Fprintf(out, "  via %s\n", r.scope.path(finding.Callable, r.names))
		}
		for _, action := range finding.Actions {
			fmt.Fprintf(out, "  %s\n", action)
		}
	}
	for _, limitation := range r.limitations {
		if command.presentation == analysePresentDefault && strings.HasPrefix(limitation, "depth ") {
			continue
		}
		fmt.Fprintf(out, "incomplete: %s\n", limitation)
	}
	if command.presentation == analysePresentInfo {
		r.writeSummary(out, true)
	}
}

// analysedFamilies names the analyses a report covers and notes the
// families that run only as compilation diagnostics or are unavailable.
func (r analyseReport) analysedFamilies() (run []string, notes []string) {
	if r.family != "" {
		return []string{r.family}, nil
	}
	for _, family := range analysisFamilies() {
		if family.Availability == analysisAvailable {
			run = append(run, family.Name)
		} else {
			notes = append(notes, fmt.Sprintf("%s: %s", family.Name, family.Limitation))
		}
	}
	return run, notes
}

// writeSummary is the concise --info block: analyses, scope, depth, target,
// counts and limitations. Verbose output already ends with the results line.
func (r analyseReport) writeSummary(out io.Writer, withResults bool) {
	run, notes := r.analysedFamilies()
	fmt.Fprintln(out, "\nanalysis summary:")
	if r.family == "" {
		fmt.Fprintf(out, "  analyses: all (%s)\n", strings.Join(run, ", "))
	} else {
		fmt.Fprintf(out, "  analyses: %s\n", strings.Join(run, ", "))
	}
	for _, note := range notes {
		fmt.Fprintf(out, "  not run alone: %s\n", note)
	}
	if r.scope == nil {
		fmt.Fprintln(out, "  scope: whole program")
	} else {
		fmt.Fprintf(out, "  scope: %s and %d reachable callable(s); the whole program was analysed\n", r.names[r.scope.root.ID], len(r.scope.reachable)-1)
	}
	fmt.Fprintf(out, "  depth: %s\n  target: %s\n", r.depth, r.target.String())
	if withResults {
		fmt.Fprintf(out, "  results: %d errors, %d unproven, %d warnings, %d advisory\n",
			r.counts[analyseClassError], r.counts[analyseClassUnproven], r.counts[analyseClassWarning], r.counts[analyseClassAdvisory])
	}
	if len(r.limitations) == 0 {
		fmt.Fprintln(out, "  limitations: none")
	}
	for _, limitation := range r.limitations {
		fmt.Fprintf(out, "  limitation: %s\n", limitation)
	}
}

// document builds the sec.analysis-report for --json. A rejected or
// interrupted run (no completed report) still produces a valid document
// whose status says so.
func (r analyseReport) document(command analyseCommand, occurrences []emittedOccurrence, summary diagnosticSummary) analysisReportDocument {
	document := analysisReportDocument{
		Format: analysisReportFormat, Version: analysisReportFormatVersion,
		Target: command.target.String(), Depth: string(command.depth), Diagnostics: occurrences,
	}
	if !r.completed {
		document.Status = "rejected"
		document.Summary = analysisReportSummary{Errors: summary.Errors, Warnings: summary.Warnings, Information: summary.Information}
		document.Limitations = []string{"analysis did not complete; see diagnostics"}
		return document
	}
	document.Status = "complete"
	if len(r.limitations) > 0 {
		document.Status = "incomplete"
	}
	for _, family := range analysisFamilies() {
		entry := analysisReportAnalysis{Name: family.Name}
		switch {
		case r.family != "" && r.family != family.Name:
			continue
		case family.Availability == analysisAvailable:
			entry.Status, entry.Complete = "executed", true
		default:
			entry.Status, entry.Note = "not-selectable", family.Limitation
		}
		document.Analyses = append(document.Analyses, entry)
	}
	if r.scope != nil {
		focus := &analysisReportFocus{Selector: r.scope.selector, Callable: string(r.scope.root.ID), Name: r.names[r.scope.root.ID]}
		for callable := range r.scope.reachable {
			focus.Reachable = append(focus.Reachable, string(callable))
		}
		sort.Strings(focus.Reachable)
		document.Focus = focus
	}
	selectedDiagnostics := []emittedOccurrence{}
	keep := map[string]bool{}
	for _, warning := range r.warnings {
		keep[fmt.Sprintf("%s:%d:%d:%s", warning.File, warning.Line, warning.Column, warning.ID)] = true
	}
	for _, occurrence := range occurrences {
		if occurrence.Severity == diagnostics.SeverityError {
			selectedDiagnostics = append(selectedDiagnostics, occurrence)
			continue
		}
		id := ""
		if occurrence.ID != nil {
			id = *occurrence.ID
		}
		if occurrence.Primary != nil && keep[fmt.Sprintf("%s:%d:%d:%s", occurrence.Primary.Span.File, occurrence.Primary.Span.Start.Line, occurrence.Primary.Span.Start.Column, id)] {
			selectedDiagnostics = append(selectedDiagnostics, occurrence)
		}
	}
	document.Diagnostics = selectedDiagnostics
	for _, finding := range r.findings {
		entry := analysisReportFinding{
			Analysis: finding.Family, Class: string(finding.Class), Rule: finding.Rule, Message: finding.Message,
			Location: occurrenceSpan{File: finding.Token.File, Start: occurrencePosition{Line: finding.Token.Line, Column: finding.Token.Column},
				End: occurrencePosition{Line: finding.Token.Line, Column: finding.Token.Column}},
			Actions: append([]string{}, finding.Actions...),
		}
		if finding.Callable != "" {
			entry.Callable = string(finding.Callable)
			if r.scope != nil {
				entry.Path = r.scope.path(finding.Callable, r.names)
			}
		}
		document.Findings = append(document.Findings, entry)
	}
	document.Limitations = append([]string{}, r.limitations...)
	document.Summary = analysisReportSummary{
		Errors: r.counts[analyseClassError], Unproven: r.counts[analyseClassUnproven], Warnings: r.counts[analyseClassWarning],
		Advisory: r.counts[analyseClassAdvisory], Information: summary.Information,
	}
	return document
}
