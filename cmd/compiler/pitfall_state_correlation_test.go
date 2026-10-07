package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestPitfallStateCorrelationCLI transports one discardability owner per
// rejected operation and the copy/paste explanation only when supported.
// Rules: rules/analysis/pitfall_analysis.md — "Option, Result, and state-correlation pitfalls",
// "Diagnostic ownership and coalescing".
func TestPitfallStateCorrelationCLI(t *testing.T) {
	file := "../../testdata/sema/pitfall_state_correlation_invalid.sec"
	for _, command := range []string{"sema", "analyse"} {
		_, output, code := runCLIForDiagnostics(t, command, file, "--diagnostic-format=json")
		doc := decodeOccurrenceDocument(t, output)
		if code != 3 || doc.Summary.Errors != 18 || len(doc.Occurrences) != 18 {
			t.Fatal(code, output)
		}
		correlated := 0
		for _, e := range doc.Occurrences {
			if e.ID == nil || *e.ID != diagnostics.NonDiscardableValue || e.Unregistered || len(e.Fixes) != 0 {
				t.Fatal(e)
			}
			for _, help := range e.Help {
				if strings.Contains(help.Text, "pitfall.state.wrong-checked-subject") {
					correlated++
					if !strings.Contains(help.Text, "different resolved binding") || !strings.Contains(help.Text, "Suggested edit:") {
						t.Fatal(help)
					}
				}
			}
		}
		if correlated != 11 {
			t.Fatal("wrong number of supported explanations", correlated, output)
		}
	}
	_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/sema/pitfall_state_correlation_valid.sec", "--diagnostic-format=json")
	valid := decodeOccurrenceDocument(t, output)
	if code != 0 || valid.Summary.Errors != 0 || len(valid.Occurrences) != 0 {
		t.Fatal(code, output)
	}
}
