package main

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestStatementMatchResultsLSP keeps Sema's mandatory result identities and
// source ranges for expression arms, blocks, nested matches and defer.
// Rules: rules/control-flow/flowcontrol_match.md §19(2)-(6);
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestStatementMatchResultsLSP(t *testing.T) {
	file := "../../testdata/sema/match_statement_results_invalid.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := sema.NewAnalyzer().Analyze(program)
	if len(errors) != 12 {
		t.Fatal(errors)
	}
	for _, err := range errors {
		d := semaDiagnostic(err, 3, string(source))
		if (d.Code != diagnostics.UnhandledMustUseResult && d.Code != diagnostics.NonDiscardableValue) || d.Severity != 1 || d.Range.Start.Line != err.Line-1 || d.Range.End.Character <= d.Range.Start.Character {
			t.Errorf("incomplete result diagnostic: %+v", d)
		}
	}
}
