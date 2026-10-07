package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestPitfallStateCorrelationLSP preserves mandatory owner presentation and
// navigation to the actual state check without emitting another diagnostic.
// Rules: rules/analysis/pitfall_analysis.md — "Option, Result, and state-correlation pitfalls",
// "Diagnostic ownership and coalescing"; rules/tooling/lsp.md — "Shared diagnostic model".
func TestPitfallStateCorrelationLSP(t *testing.T) {
	file := "../../testdata/sema/pitfall_state_correlation_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.NewWithFile(string(data), file)).ParseProgram()
	errors := sema.NewAnalyzer().Analyze(program)
	if len(errors) != 18 {
		t.Fatal(errors)
	}
	correlated := 0
	for _, e := range errors {
		d := semaDiagnostic(e, 3, string(data))
		if d.Severity != 1 || d.Code != diagnostics.NonDiscardableValue {
			t.Fatal(d)
		}
		if strings.Contains(d.Message, "pitfall.state.wrong-checked-subject") {
			correlated++
			if len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.Range.Start.Line != e.PreviousLine-1 || !strings.Contains(d.Message, "Suggested edit:") {
				t.Fatal(d)
			}
		}
	}
	if correlated != 11 {
		t.Fatal(correlated)
	}
}
