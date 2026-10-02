package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"sec/internal/ast"
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
// Individual analysis selection is not offered until its CLI spelling is
// decided (missing-decisions.yaml MD-017).
type analyseCommand struct {
	inputs []string
	target CompilerTarget
}

// parseAnalyseCommandArgs accepts `sec analyse [--all] [--target <os-arch>]
// <inputs>...`.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — `sec analyse` and `sec analyse --all`
//   - rules/analysis/pitfall_analysis.md — sec analyse runs all available analyses by default
func parseAnalyseCommandArgs(args []string, host CompilerTarget) (analyseCommand, error) {
	command := analyseCommand{target: host}
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; {
		case arg == "--all":
		case arg == "--target":
			if index+1 >= len(args) {
				return analyseCommand{}, fmt.Errorf("--target requires <os-arch>")
			}
			index++
			target, ok := parseCompilerTarget(args[index])
			if !ok {
				return analyseCommand{}, fmt.Errorf("invalid target %q", args[index])
			}
			command.target = target
		case strings.HasPrefix(arg, "-"):
			return analyseCommand{}, fmt.Errorf("unknown analyse option %s", arg)
		default:
			command.inputs = append(command.inputs, arg)
		}
	}
	if len(command.inputs) == 0 {
		return analyseCommand{}, fmt.Errorf("analyse requires at least one <file.sec|dir|glob>")
	}
	return command, nil
}

// runAnalyseCommand performs analysis-only compilation: the same source, plan
// and Sema facts as build/check at Deep depth, without Semantic IR or code
// generation, and prints the canonical analysis report.
//
// Rules:
//   - rules/compiler/compiler_pipeline.md — § 65(1–3) analysis-only mode
//   - rules/compiler/compiler_analysis.md — § 61(1–3) `sec analyse`; § 62 analysis modes
//   - rules/analysis/pitfall_analysis.md — sec analyse; Deep output
//   - rules/analysis/parameter_usage_analysis.md — `sec analyse`; Deep output
func runAnalyseCommand(args []string, out io.Writer) {
	command, err := parseAnalyseCommandArgs(args, hostCompilerTarget())
	if err != nil {
		reportToolError("usage", "%v", err)
		printUsage()
		exitCLI(1)
	}
	program, summary := parseSourceInputs(command.inputs, command.target, true)
	selected := analyseSelectedFiles(program)
	analyzer := analyzeProgramWithSourcesAtDepth(program, command.target, collectSourceRootsFromInputs(command.inputs), summary, true, sema.AnalysisDeep)
	report := buildAnalyseReport(analyzer, selected, command.target)
	report.write(out)
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
	sections []analyseSection
	counts   map[analyseReportClass]int
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
func buildAnalyseReport(analyzer *sema.Analyzer, selected map[string]bool, target CompilerTarget) analyseReport {
	report := analyseReport{target: target, counts: map[analyseReportClass]int{}}
	for _, warning := range analyzer.Warnings() {
		if selected[warning.File] {
			report.counts[analyseClassWarning]++
		}
	}
	inSelection := func(token lexer.Token) bool { return selected[token.File] }
	graph := analyzer.CallGraph()
	names := map[sema.CallableID]string{}
	for _, node := range graph.Nodes() {
		names[node.ID] = analyseCallableName(node)
	}
	nodes := []sema.CallableNode{}
	for _, node := range graph.Nodes() {
		if inSelection(node.Declaration) {
			nodes = append(nodes, node)
		}
	}
	report.addCallGraph(graph, nodes, names, inSelection)
	report.addEffects(graph, nodes, names)
	report.addEscape(analyzer.EscapeAnalysis(), inSelection)
	report.addParameterUsage(analyzer.ParameterUsageAnalysis(), inSelection)
	report.addPitfalls(analyzer.PitfallAnalysis(), inSelection)
	return report
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

// addParameterUsage reports the semantic demand of each declared parameter.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — `sec analyse`; Deep output (declared capability, required demand)
func (r *analyseReport) addParameterUsage(usage *sema.ParameterUsageAnalysis, inSelection func(lexer.Token) bool) {
	section := r.section("parameter usage")
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
// fixes versus suggested edits, and rules that were not evaluated.
// Rule: rules/analysis/pitfall_analysis.md — sec analyse Deep output.
func (r *analyseReport) addPitfalls(pitfalls *sema.PitfallAnalysis, inSelection func(lexer.Token) bool) {
	section := r.section("pitfalls")
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
		class := pitfallReportClass(finding)
		r.counts[class]++
		section.add("%s %s at %s: %s (%s, %s, owning rule %s)", class, finding.Rule, position, finding.Subject.Expression, finding.Classification, finding.Confidence, finding.OwningRule)
		for _, evidence := range finding.EvidenceFor {
			section.add("  evidence %s: %s", evidence.Strength, evidence.Fact)
		}
		for _, evidence := range finding.EvidenceAgainst {
			section.add("  evidence against %s: %s", evidence.Strength, evidence.Fact)
		}
		for _, action := range finding.Actions {
			section.add("  %s: %s", action.Kind, action.Title)
		}
	}
	notEvaluated := []string{}
	for _, evaluation := range pitfalls.Evaluations() {
		if evaluation.State == sema.PitfallStateNotEvaluated || evaluation.State == sema.PitfallStatePending {
			notEvaluated = append(notEvaluated, fmt.Sprintf("%s (%s)", evaluation.Rule, evaluation.State))
		}
	}
	sort.Strings(notEvaluated)
	for _, rule := range notEvaluated {
		section.add("not evaluated %s", rule)
	}
}

func (r analyseReport) write(out io.Writer) {
	fmt.Fprintf(out, "analysis report (depth %s, target %s)\n", sema.AnalysisDeep, r.target.String())
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
