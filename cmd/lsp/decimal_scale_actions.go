package main

import (
	"fmt"
	"reflect"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// decimalScaleCodeActions materializes A2012's selected scale before its first
// range. Client text is never proof: the current target/overlay snapshot and
// exact replacement must analyze successfully and retain the decimal grid,
// scale-bearing range bounds and synthesized default.
// Rules: rules/tooling/diagnostics.md — §31A;
// rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — §15(6).
func decimalScaleCodeActions(uri, text string, reported []diagnostic, overlay sourceOverlay) []codeAction {
	relevant := false
	for _, d := range reported {
		if d.Code == diagnostics.InferredDecimalScale {
			relevant = true
		}
	}
	if !relevant {
		return nil
	}
	path := pathFromURI(uri)
	parsed := parser.New(lexer.NewWithFile(text, path)).Parse()
	if parsed.Fatal || len(parsed.Diagnostics) != 0 {
		return nil
	}
	// Capture the source contracts before workspace preparation qualifies names.
	sites := map[string]lexer.Token{}
	for _, stmt := range parsed.Program.Statements {
		decl, ok := stmt.(*ast.TypeDeclStatement)
		if !ok || decl.Name == nil || len(decl.GenericParameters) != 0 {
			continue
		}
		contracts := []ast.Contract{decl.Contract}
		if list, ok := decl.Contract.(*ast.ContractList); ok {
			contracts = list.Contracts
		}
		for _, contract := range contracts {
			if r, ok := contract.(*ast.RangeContract); ok {
				sites[decl.Name.Value] = r.Token
				break
			}
		}
	}
	prepareProgramForLSP(parsed.Program, path, overlay)
	analyzer := newLSPAnalyzerWithOverlay(uri, parsed.Program, overlay)
	if errs := analyzeCompilerSnapshot(analyzer, parsed.Program, overlay); len(errs) != 0 {
		return nil
	}
	var actions []codeAction
	for _, warning := range analyzer.Warnings() {
		if warning.ID != diagnostics.InferredDecimalScale || normalizedSourcePath(warning.File) != normalizedSourcePath(path) {
			continue
		}
		current := semaDiagnosticWithSources(warning, 3, uri, text, overlay)
		if !currentUnitDiagnostic(current, reported) {
			continue
		}
		name := warning.Arguments["type"]
		token, ok := sites[name]
		if !ok {
			continue
		}
		before, ok := analyzer.Types()[name]
		if !ok {
			continue
		}
		scale, ok := sema.FixedDecimalScaleOf(before)
		if !ok {
			continue
		}
		at := diagnosticTokenStart(text, token)
		offset := lineCharToOffset(text, at.Line, at.Character)
		if offset < 0 || offset+5 > len(text) || text[offset:offset+5] != "range" {
			continue
		}
		replacement := fmt.Sprintf("scale %d ", scale)
		edited := text[:offset] + replacement + text[offset:]
		checked := parser.New(lexer.NewWithFile(edited, path)).Parse()
		if checked.Fatal || len(checked.Diagnostics) != 0 {
			continue
		}
		updated := sourceOverlay{Sources: map[string]string{}, Targets: overlay.Targets, workContext: overlay.workContext}
		for file, source := range overlay.Sources {
			updated.Sources[file] = source
		}
		updated.Sources[normalizedSourcePath(path)] = edited
		prepareProgramForLSP(checked.Program, path, updated)
		next := newLSPAnalyzerWithOverlay(uri, checked.Program, updated)
		if len(analyzeCompilerSnapshot(next, checked.Program, updated)) != 0 {
			continue
		}
		after, ok := next.Types()[name]
		if !ok || !decimalScaleFixEquivalent(before, after) {
			continue
		}
		actions = append(actions, codeAction{Title: fmt.Sprintf("Declare scale %d explicitly", scale), Kind: "quickfix", Diagnostics: []diagnostic{current}, Edit: workspaceEdit{Changes: map[string][]textEdit{uri: {{Range: lspRange{Start: at, End: at}, NewText: replacement}}}}})
	}
	return actions
}

// decimalScaleFixEquivalent checks the canonical grid, exact scale-bearing
// range conjunction and default after inserting a scale clause. Source tokens
// and inference explanation differ; they are not numeric type semantics.
// Rules: rules/types/contracts.md — §16; rules/types/default_values.md — §65A;
// rules/tooling/diagnostics.md — §31A.
func decimalScaleFixEquivalent(before, after sema.Type) bool {
	s, ok := sema.FixedDecimalScaleOf(before)
	t, fixed := sema.FixedDecimalScaleOf(after)
	if !ok || !fixed || s != t || before.Kind != after.Kind || before.Name != after.Name {
		return false
	}
	normalize := func(typ sema.Type) []sema.Contract {
		var result []sema.Contract
		for _, contract := range typ.Contracts {
			switch c := contract.(type) {
			case sema.ScaleContract:
				continue
			case sema.RangeContract:
				c.Token = lexer.Token{}
				result = append(result, c)
			default:
				return nil
			}
		}
		return result
	}
	a, b := normalize(before), normalize(after)
	if a == nil || b == nil || !reflect.DeepEqual(a, b) {
		return false
	}
	old, new := sema.DefaultValueOf(before), sema.DefaultValueOf(after)
	old.Value.Token, new.Value.Token = lexer.Token{}, lexer.Token{}
	return reflect.DeepEqual(old, new)
}
