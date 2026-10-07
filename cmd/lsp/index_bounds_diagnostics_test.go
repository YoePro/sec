package main

import (
	"os"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestIndexBoundsDiagnosticsLSP consumes the same coalesced mandatory errors,
// evidence, source ranges and suggestions as the compiler with severity overrides.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing";
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestIndexBoundsDiagnosticsLSP(t *testing.T) {
	file := "../../testdata/sema/index_bounds_diagnostics_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errs := sema.NewAnalyzer().Analyze(program)
	if len(errs) != 4 {
		t.Fatal(errs)
	}
	for _, e := range errs {
		d := semaDiagnostic(e, 3, string(data))
		if d.Code != diagnostics.IndexOutOfBounds || d.Severity != 1 || !strings.Contains(d.Message, "Invalid:") || !strings.Contains(d.Message, "pitfall.bounds.") || !strings.Contains(d.Message, "Suggested edit:") || d.Range.Start.Line != e.Line-1 || d.Range.Start.Character != e.Column-1 {
			t.Fatal(d)
		}
		if e.RelatedLabel == "bounds evidence" && (len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.Range.Start.Line != e.PreviousLine-1) {
			t.Fatal(d)
		}
	}
}
