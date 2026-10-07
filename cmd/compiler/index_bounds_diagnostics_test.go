package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestIndexBoundsDiagnosticsCLI requires one mandatory bounds occurrence per
// operation, with pitfall evidence and suggested edits in every semantic gate.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing";
// rules/tooling/diagnostics.md — §§8–10, 14.
func TestIndexBoundsDiagnosticsCLI(t *testing.T) {
	file := "../../testdata/sema/index_bounds_diagnostics_invalid.sec"
	for _, command := range []string{"sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		t.Run(command, func(t *testing.T) {
			args := []string{command, file}
			if strings.HasPrefix(command, "emit-") {
				args = append(args, "-o", "-")
			}
			_, output, code := runCLIForDiagnostics(t, append(args, "--diagnostic-format=json")...)
			doc := decodeOccurrenceDocument(t, output)
			if code != 3 || doc.Summary.Errors != 4 || doc.Summary.Warnings != 0 || len(doc.Occurrences) != 4 {
				t.Fatal(code, output)
			}
			for _, e := range doc.Occurrences {
				if e.ID == nil || *e.ID != diagnostics.IndexOutOfBounds || e.Unregistered || e.Arguments["proof_state"] != "Invalid" || e.Primary == nil || e.Primary.Span.File != file || len(e.Help) != 1 || len(e.Fixes) != 0 || !strings.Contains(e.Help[0].Text, "pitfall.bounds.") {
					t.Fatal(e)
				}
			}
			_, human, humanCode := runCLIForDiagnostics(t, args...)
			if humanCode != 3 || !strings.Contains(human, "Suggested edit: use the canonical half-open range") || !strings.Contains(human, "Len is the element count") || strings.Count(human, "[S1134]") != 4 {
				t.Fatal(humanCode, human)
			}
		})
	}
}
