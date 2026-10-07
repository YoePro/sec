package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestPitfallDiagnosticOwnershipLSP keeps one mandatory occurrence, its exact
// range and related navigation while transporting the supporting explanation.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing";
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestPitfallDiagnosticOwnershipLSP(t *testing.T) {
	file := "../../testdata/sema/pitfall_diagnostic_ownership_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.NewWithFile(string(data), file)).ParseProgram()
	errors := sema.NewAnalyzer().Analyze(program)
	if len(errors) != 3 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		d := semaDiagnostic(e, 3, string(data))
		if d.Severity != 1 || d.Code != e.ID || strings.Count(d.Message, "pitfall.") != 1 || !strings.Contains(d.Message, "Suggested edit:") || d.Range.Start.Line != e.Line-1 {
			t.Fatal(d)
		}
		if e.PreviousFile != "" && len(d.RelatedInformation) != 1 {
			t.Fatal("missing canonical evidence navigation", d)
		}
	}
}
