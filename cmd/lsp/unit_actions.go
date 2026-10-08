package main

import (
	"path/filepath"
	"sec/internal/ast"
	"sec/internal/lexer"
	unitactions "sec/internal/lsp/features/units"
	lspserver "sec/internal/lsp/server"
	"sec/internal/parser"
	"sort"
	"strings"
)

// unitConversionCodeActions offers the compiler-proven explicit unit
// conversion Sema recorded for a rejected value, attached to the reported
// diagnostic at that value. The LSP never derives a conversion itself and
// never invents a factor, exchange rate, or rounding policy.
//
// Rules:
//   - rules/tooling/lsp.md — "Unit actions"
//   - rules/types/units.md — "LSP requirements"
func unitConversionCodeActions(uri string, text string, reported []diagnostic, overlay sourceOverlay) []codeAction {
	if len(reported) == 0 {
		return nil
	}
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return nil
	}
	path := pathFromURI(uri)
	prepareProgramForLSP(program, path, overlay)
	analyzer := newLSPAnalyzerWithOverlay(uri, program, overlay)
	analyzer.Analyze(program)
	actions := []codeAction{}
	baseline := analyze(uri, text, overlay)
	for _, suggestion := range analyzer.UnitConversionSuggestions() {
		if suggestion.File != "" && path != "" && normalizedSourcePath(suggestion.File) != normalizedSourcePath(path) {
			continue
		}
		start := diagnosticTokenStart(text, lexer.Token{Line: suggestion.Line, Column: suggestion.Column})
		end := diagnosticTokenStart(text, lexer.Token{Line: suggestion.EndLine, Column: suggestion.EndColumn})
		for _, reportedDiagnostic := range reported {
			if reportedDiagnostic.Range.Start != start || !currentUnitDiagnostic(reportedDiagnostic, baseline) {
				continue
			}
			edits := []textEdit{{Range: lspRange{Start: start, End: end}, NewText: suggestion.Replacement}}
			if !unitActionValid(uri, text, overlay, baseline, reportedDiagnostic, edits) {
				continue
			}
			title := "Convert explicitly to " + suggestion.Target + " with " + suggestion.Replacement
			if suggestion.Kind == "declared-function" {
				title = "Use declared conversion " + suggestion.Replacement
			}
			if suggestion.Kind == "in-scope-factor" {
				title = "Convert with in-scope factor: " + suggestion.Replacement
			}
			if suggestion.Kind == "exact-carrier" {
				title = "Change carrier exactly with " + suggestion.Replacement
			}
			actions = append(actions, codeAction{
				Title:       title,
				Kind:        "quickfix",
				Diagnostics: []diagnostic{reportedDiagnostic},
				Edit: workspaceEdit{Changes: map[string][]textEdit{
					uri: {{Range: lspRange{Start: start, End: end}, NewText: suggestion.Replacement}},
				}},
			})
			break
		}
	}
	return append(actions, unitImportCodeActions(uri, text, reported, overlay, baseline)...)
}

// currentUnitDiagnostic rejects stale or unrelated diagnostics supplied by a client.
// Rules: rules/tooling/lsp.md — Snapshots and Safe fixes.
func currentUnitDiagnostic(reported diagnostic, current []diagnostic) bool {
	for _, d := range current {
		if d.Code == reported.Code && d.Range == reported.Range && d.Message == reported.Message {
			return true
		}
	}
	return false
}

// unitActionValid reparses/reanalyzes the exact edit with the original target and
// overlays, proving that the owning error disappears without adding any errors.
// Existing unrelated errors may remain while a local unit mismatch is repaired.
// Rules: rules/tooling/lsp.md — Unit actions, Safe fixes, Snapshots;
// rules/types/units.md — No hidden precision loss and Contracts on unit-bearing named types.
func unitActionValid(uri, text string, overlay sourceOverlay, baseline []diagnostic, owner diagnostic, edits []textEdit) bool {
	edited := applyTextEdits(text, edits)
	if edited == text {
		return false
	}
	updated := sourceOverlay{Sources: map[string]string{}, Targets: overlay.Targets}
	for file, source := range overlay.Sources {
		updated.Sources[file] = source
	}
	updated.Sources[normalizedSourcePath(pathFromURI(uri))] = edited
	before := map[string]int{}
	for _, d := range baseline {
		if d.Severity == 1 {
			before[d.Code+"\x00"+d.Message]++
		}
	}
	after := map[string]int{}
	for _, d := range analyze(uri, edited, updated) {
		if d.Severity == 1 {
			after[d.Code+"\x00"+d.Message]++
		}
	}
	key := owner.Code + "\x00" + owner.Message
	if before[key] == 0 || after[key] >= before[key] {
		return false
	}
	for key, count := range after {
		if count > before[key] {
			return false
		}
	}
	return true
}

// unitImportCodeActions indexes visible project and standard-library unit
// declarations, then asks the compiler to validate the exact import/qualification
// edit. Private, unrelated, already-resolved, stale and invalid imports are rejected.
// Rules: rules/tooling/lsp.md — Unit actions, Snapshots, Safe fixes;
// rules/types/units.md — LSP requirements; rules/projects/modules.md — §§7–9.
func unitImportCodeActions(uri, text string, reported []diagnostic, overlay sourceOverlay, baseline []diagnostic) []codeAction {
	hasUnknown := false
	for _, d := range reported {
		if strings.Contains(d.Message, "unknown unit ") && currentUnitDiagnostic(d, baseline) {
			hasUnknown = true
		}
	}
	if !hasUnknown {
		return nil
	}
	sourcePath := pathFromURI(uri)
	root := findProjectRoot(sourcePath)
	parsed := parser.New(lexer.NewWithFile(text, sourcePath)).Parse()
	if parsed.Fatal || len(parsed.Diagnostics) > 0 {
		return nil
	}
	type candidate struct{ module, name, path string }
	candidates := []candidate{}
	seen := map[string]bool{}
	paths := []string{}
	if filepath.Dir(root) != root {
		paths = workspaceSourcePaths([]string{root}, overlay)
	}
	libraryRoot := filepath.Join(findSecSourceRoot(sourcePath), "sec", "stdlib", "unit")
	paths = append(paths, workspaceSourcePaths([]string{libraryRoot}, overlay)...)
	sort.Strings(paths)
	for _, file := range paths {
		normalized := normalizedSourcePath(file)
		if normalized == normalizedSourcePath(sourcePath) {
			continue
		}
		slash := filepath.ToSlash(normalized)
		if strings.Contains(slash, "/manual/") || strings.Contains(slash, "/sec/") && strings.Contains(slash, "/sec/stdlib/") && !strings.HasPrefix(normalized, normalizedSourcePath(libraryRoot)+string(filepath.Separator)) {
			continue
		}
		if strings.Contains(slash, "/sec/core/") || strings.Contains(slash, "/sec/platform/") {
			continue
		}
		program, ok := lspserver.ParseSource(file, overlay.Sources)
		if !ok || !lspserver.ProgramMatchesTarget(program, lspActiveTarget(parsed.Program, sourcePath, overlay)) {
			continue
		}
		module := lspserver.ProgramModule(program)
		if module == "" || module == lspserver.ProgramModule(parsed.Program) {
			continue
		}
		relative, err := filepath.Rel(root, filepath.Dir(file))
		if err != nil {
			continue
		}
		importPath := filepath.ToSlash(relative)
		if strings.HasPrefix(normalized, normalizedSourcePath(libraryRoot)+string(filepath.Separator)) || normalizedSourcePath(filepath.Dir(file)) == normalizedSourcePath(libraryRoot) {
			relative, err = filepath.Rel(filepath.Join(findSecSourceRoot(sourcePath), "sec", "stdlib"), filepath.Dir(file))
			if err != nil {
				continue
			}
			importPath = filepath.ToSlash(relative)
		}
		if importPath == "." {
			importPath = strings.TrimSuffix(filepath.Base(file), ".sec")
		}
		if !canonicalLSPImportPath(importPath) {
			continue
		}
		for _, statement := range program.Statements {
			unit, ok := statement.(*ast.UnitDeclStatement)
			if !ok || unit.Name == nil || strings.HasPrefix(unit.Name.Value, "_") {
				continue
			}
			key := importPath + "\x00" + unit.Name.Value
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, candidate{module, unit.Name.Value, importPath})
		}
	}
	actions := []codeAction{}
	offered := map[string]bool{}
	for _, owner := range reported {
		if !currentUnitDiagnostic(owner, baseline) {
			continue
		}
		line := strings.Split(owner.Message, "\n")[0]
		index := strings.Index(line, "unknown unit ")
		if index < 0 {
			continue
		}
		words := strings.Fields(line[index+len("unknown unit "):])
		if len(words) == 0 {
			continue
		}
		unknown := words[0]
		for _, candidate := range candidates {
			key := unknown + "\x00" + candidate.path
			if offered[key] {
				continue
			}
			name := unknown
			if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
				name = name[dot+1:]
			}
			if candidate.name != name {
				continue
			}
			proposals := unitactions.ImportEdits(text, parsed.Program, unknown, candidate.module, candidate.name, candidate.path)
			edits := []textEdit{}
			for _, proposal := range proposals {
				edits = append(edits, textEdit{Range: lspRange{Start: offsetPosition(text, proposal.Start), End: offsetPosition(text, proposal.End)}, NewText: proposal.Text})
			}
			if len(edits) == 0 || !unitActionValid(uri, text, overlay, baseline, owner, edits) {
				continue
			}
			offered[key] = true
			actions = append(actions, codeAction{Title: "Import " + candidate.path + " for unit " + candidate.name, Kind: "quickfix", Diagnostics: []diagnostic{owner}, Edit: workspaceEdit{Changes: map[string][]textEdit{uri: edits}}})
		}
	}
	return actions
}
