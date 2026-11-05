package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/project"
	"sec/internal/sema"
)

type analyseConformanceTarget struct {
	target     CompilerTarget
	facts      []sema.InterfaceConformanceFact
	errors     []sema.Error
	interfaces []lexer.Token
	selected   map[string]bool
}

// analysisConformance is a presentation of explicit compiler-owned conformance,
// not a second implementation check. Missing and unproven facts stay distinct
// from validity, just as in the LSP interface and implementing-type lenses.
// Rules: rules/tooling/lsp.md — Interface conformance code lens;
// rules/declarations/interfaces.md §§4–6,12; rules/compiler/semantic_ir.md §109.
type analysisConformance struct {
	Target      string         `json:"target"`
	Module      string         `json:"module"`
	Type        string         `json:"type"`
	Interface   string         `json:"interface"`
	State       string         `json:"state"`
	Declaration occurrenceSpan `json:"declaration"`
	token       lexer.Token
	Failures    []analysisConformanceFailure `json:"failures"`
}

type analysisConformanceFailure struct {
	Member   string         `json:"member"`
	ID       string         `json:"id"`
	Message  string         `json:"message"`
	Location occurrenceSpan `json:"location"`
}

// analyseConformanceTargets uses the ordinary CLI compilation preparation and
// each selected target's scalar plan. A directed/explicit target limits the
// view; otherwise same-module sibling directives select the target matrix.
// Retained Sema errors are reported by addConformances, never suppressed.
// Rules: rules/tooling/lsp.md — Interface conformance code lens;
// rules/platform/platform_model.md — Source selection by target;
// rules/compiler/compiler_pipeline.md §26; rules/tooling/cli.md §§5–6.
func analyseConformanceTargets(command analyseCommand, initial *ast.Program, summary diagnosticSummary) (*sema.Analyzer, []analyseConformanceTarget) {
	versions := validateDiscoveredProjectManifests(collectSourceRootsFromInputs(command.inputs))
	if versions.Errors > 0 {
		printDiagnosticSummary(versions)
		exitCLI(3)
	}
	targets := map[string]CompilerTarget{command.target.String(): command.target}
	directed := false
	for _, input := range command.inputs {
		info, err := os.Stat(input)
		if err != nil || info.IsDir() {
			continue
		}
		parsed, counts := parseSourceFileWithDiagnostics(input)
		if counts.Errors != 0 {
			continue
		}
		for _, stmt := range parsed.Program.Statements {
			if _, ok := stmt.(*ast.TargetDirective); ok {
				directed = true
			}
		}
	}
	if !command.targetExplicit && !directed {
		discovered := map[string]CompilerTarget{}
		seenModules := map[string]bool{}
		for _, stmt := range initial.Statements {
			module, ok := stmt.(*ast.ModuleStatement)
			if !ok || module.Token.File == "" {
				continue
			}
			moduleKey := filepath.Dir(module.Token.File) + "\x00" + module.Path
			if seenModules[moduleKey] {
				continue
			}
			seenModules[moduleKey] = true
			if err := addConformanceProjectTargets(module.Token.File, discovered); err != nil {
				reportToolError("target", "%v", err)
				exitCLI(1)
			}
			files, _ := filepath.Glob(filepath.Join(filepath.Dir(module.Token.File), "*.sec"))
			for _, file := range files {
				if strings.HasSuffix(file, "_test.sec") {
					continue
				}
				parsed, _ := parseSourceFileWithDiagnostics(file)
				if programModulePath(parsed.Program) != module.Path {
					continue
				}
				for _, candidate := range parsed.Program.Statements {
					if directive, ok := candidate.(*ast.TargetDirective); ok {
						arch := directive.Arch
						if arch == "any" || arch == "" {
							arch = command.target.Arch
						}
						target, valid := parseCompilerTarget(directive.OS + "-" + arch)
						if valid {
							discovered[target.String()] = target
						}
					}
				}
			}
		}
		if len(discovered) > 0 {
			targets = discovered
		}
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// Keep the requested target's ordinary analysis as the main report when it
	// belongs to the matrix. Other-target warnings are not silently attributed
	// to this primary target.
	if _, ok := targets[command.target.String()]; ok {
		for i, key := range keys {
			if key == command.target.String() {
				keys = append([]string{key}, append(keys[:i], keys[i+1:]...)...)
				break
			}
		}
	}
	var primary *sema.Analyzer
	var result []analyseConformanceTarget
	roots := collectSourceRootsFromInputs(command.inputs)
	for index, key := range keys {
		target := targets[key]
		program := initial
		counts := summary
		if index != 0 || target != command.target {
			program, counts = parseSourceInputs(command.inputs, target, true)
		}
		selected := analyseSelectedFiles(program)
		// Capture requested interfaces before resolving trusted core/imports.
		var interfaces []lexer.Token
		for _, stmt := range program.Statements {
			if decl, ok := stmt.(*ast.InterfaceDeclaration); ok && decl.Name != nil {
				interfaces = append(interfaces, decl.Name.Token)
			}
		}
		analyzer, errors := analyzeProgramWithSourcesForReport(program, target, roots, counts, false, command.depth, true)
		if primary == nil {
			primary = analyzer
		}
		result = append(result, analyseConformanceTarget{target: target, facts: analyzer.InterfaceConformances(), errors: errors, interfaces: interfaces, selected: selected})
	}
	return primary, result
}

// addConformances projects the same explicit facts as code lenses. The union
// of declared type/interface pairs makes absent declarations visible on other
// targets; exact interface declaration and type module identities prevent
// same-name collisions. Every mandatory target error remains a diagnostic.
// Rules: rules/tooling/lsp.md — Interface conformance code lens, Unknown facts;
// rules/declarations/interfaces.md §§4–6,12; rules/tooling/cli.md §§5–6.
func (r *analyseReport) addConformances(targets []analyseConformanceTarget) {
	if len(targets) == 0 {
		return
	}
	type key struct {
		module, name, iface, file string
		line, column              int
	}
	pairs := map[key]sema.InterfaceConformanceFact{}
	requested := map[string]bool{}
	for _, target := range targets {
		for _, token := range target.interfaces {
			requested[token.File] = true
		}
	}
	identity := func(fact sema.InterfaceConformanceFact) key {
		return key{fact.TypeModule, fact.TypeName, fact.Interface, fact.InterfaceDeclaration.File, fact.InterfaceDeclaration.Line, fact.InterfaceDeclaration.Column}
	}
	for _, target := range targets {
		for _, fact := range target.facts {
			if requested[fact.InterfaceDeclaration.File] || target.selected[fact.TypeDeclaration.File] {
				pairs[identity(fact)] = fact
			}
		}
	}
	section := r.section("interface conformance")
	for _, target := range targets {
		for _, err := range target.errors {
			err.Message = "[" + target.target.String() + "] " + err.Message
			err.Severity = diagnostics.SeverityError
			r.warnings = append(r.warnings, err)
			r.counts[analyseDiagnosticClass(err)]++
		}
		seen := map[key]bool{}
		for _, fact := range target.facts {
			k := identity(fact)
			if _, wanted := pairs[k]; !wanted {
				continue
			}
			seen[k] = true
			entry := analysisConformance{Target: target.target.String(), Module: fact.TypeModule, Type: fact.TypeName, Interface: fact.Interface, State: string(fact.State), Declaration: tokenSpan(fact.TypeDeclaration), token: fact.TypeDeclaration, Failures: []analysisConformanceFailure{}}
			for _, failure := range fact.Failures {
				entry.Failures = append(entry.Failures, analysisConformanceFailure{Member: failure.Member, ID: failure.Diagnostic.ID, Message: failure.Diagnostic.Message, Location: tokenSpan(lexer.Token{File: failure.Diagnostic.File, Line: failure.Diagnostic.Line, Column: failure.Diagnostic.Column, EndLine: failure.Diagnostic.EndLine, EndColumn: failure.Diagnostic.EndColumn})})
			}
			r.conformances = append(r.conformances, entry)
		}
		for k, fact := range pairs {
			if seen[k] {
				continue
			}
			r.conformances = append(r.conformances, analysisConformance{Target: target.target.String(), Module: fact.TypeModule, Type: fact.TypeName, Interface: fact.Interface, State: "missing", Declaration: tokenSpan(fact.InterfaceDeclaration), token: fact.InterfaceDeclaration, Failures: []analysisConformanceFailure{}})
		}
		// An interface with no explicit implementer anywhere still has a lens;
		// report its absence instead of presenting an empty matrix as success.
		for _, token := range target.interfaces {
			found := false
			for k := range pairs {
				if k.file == token.File && k.line == token.Line && k.column == token.Column {
					found = true
					break
				}
			}
			if !found {
				r.conformances = append(r.conformances, analysisConformance{Target: target.target.String(), Interface: token.Lexeme, State: "missing", Declaration: tokenSpan(token), token: token, Failures: []analysisConformanceFailure{}})
			}
		}
	}
	sort.Slice(r.conformances, func(i, j int) bool {
		a, b := r.conformances[i], r.conformances[j]
		return fmt.Sprint(a.Module, "\x00", a.Type, "\x00", a.Interface, "\x00", a.Target, "\x00", a.Declaration.File, a.Declaration.Start.Line, a.Declaration.Start.Column) < fmt.Sprint(b.Module, "\x00", b.Type, "\x00", b.Interface, "\x00", b.Target, "\x00", b.Declaration.File, b.Declaration.Start.Line, b.Declaration.Start.Column)
	})
	for _, entry := range r.conformances {
		members := []string{}
		for _, failure := range entry.Failures {
			members = append(members, failure.Member)
		}
		suffix := ""
		if len(members) > 0 {
			suffix = " (" + strings.Join(members, ", ") + ")"
		}
		section.add("%s: %s: %s%s", entry.Target, conformanceLabel(entry), entry.State, suffix)
		if entry.State == "missing" || entry.State == string(diagnostics.ProofUnproven) {
			r.finding(analyseFinding{Family: "interface-conformance", Class: analyseClassUnproven, Rule: "interface.conformance", Token: entry.token, Message: fmt.Sprintf("%s: %s: %s", entry.Target, conformanceLabel(entry), entry.State)})
			r.counts[analyseClassUnproven]++
		}
	}
}

// addConformanceProjectTargets adds the owning logical Target's configured
// platform Variants, deduplicating shared physical targets exactly as lenses do.
// The manifest reader and TargetForSource own parsing and source membership.
// Rules: rules/projects/projects.md §§17,19,49; rules/tooling/lsp.md — Target-aware analysis.
func addConformanceProjectTargets(source string, selected map[string]CompilerTarget) error {
	path, found := findNearestProjectManifest(filepath.Dir(source))
	if !found {
		return nil
	}
	manifest, err := project.LoadManifest(path)
	if err != nil {
		// The ordinary CLI pipeline owns mandatory manifest diagnostics.
		// Do not replace those registered errors with a target-reader error.
		return nil
	}
	absolute, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	owner, found := project.TargetForSource(manifest.Targets(), manifest.Facts().ProjectRoot, absolute)
	if !found {
		return nil
	}
	document, err := project.ParseManifestDocument(manifest.Text())
	if err != nil {
		return err
	}
	table, err := document.Table("target", owner.Name)
	if err != nil {
		return err
	}
	variants, _ := table["variants"].([]any)
	for _, value := range variants {
		name, ok := value.(string)
		if !ok {
			return fmt.Errorf("Target %s has an invalid Variant", owner.Name)
		}
		variant, err := document.Table("variant", name)
		if err != nil {
			return err
		}
		osName, _ := variant["os"].(string)
		arch, _ := variant["arch"].(string)
		target, ok := parseCompilerTarget(osName + "-" + arch)
		if !ok {
			return fmt.Errorf("Variant %s has an invalid platform", name)
		}
		selected[target.String()] = target
	}
	return nil
}

// conformanceLabel presents a type/interface relationship or an interface with
// no declared implementer, retaining module qualification for type identities.
// Rules: rules/tooling/lsp.md — Interface conformance code lens.
func conformanceLabel(entry analysisConformance) string {
	if entry.Type == "" {
		return "interface " + entry.Interface + " (no declared implementation)"
	}
	name := entry.Type
	if entry.Module != "" {
		name = entry.Module + "." + name
	}
	return name + " implements " + entry.Interface
}
