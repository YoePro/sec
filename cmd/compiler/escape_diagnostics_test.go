package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestEscapeDiagnosticsCLI preserves stable escape IDs, provenance and help
// through both public diagnostic formats and every semantic/lowering gate.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic quality";
// rules/tooling/diagnostics.md — §§9, 14, 30.
func TestEscapeDiagnosticsCLI(t *testing.T) {
	path := "../../testdata/sema/escape_diagnostics_invalid.sec"
	for _, command := range []string{"sema", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		t.Run(command, func(t *testing.T) {
			args := []string{command, path}
			if command != "sema" {
				args = append(args, "-o", "-")
			}
			_, output, code := runCLIForDiagnostics(t, append(args, "--diagnostic-format=json")...)
			doc := decodeOccurrenceDocument(t, output)
			if code != 3 || doc.Summary.Errors != 9 || len(doc.Occurrences) != 9 {
				t.Fatal(code, output)
			}
			for _, e := range doc.Occurrences {
				if e.ID == nil || !strings.HasPrefix(*e.ID, "S11") || e.Unregistered || e.Source != "sema" || e.Primary == nil || e.Primary.Span.File != path || len(e.Help) != 1 || len(e.Notes) < 2 {
					t.Fatal(e)
				}
				if *e.ID != diagnostics.EscapeVariadicPack && (len(e.Related) < 1 || e.Related[0].Span.File != path) {
					t.Fatal(e)
				}
			}
			_, human, humanCode := runCLIForDiagnostics(t, args...)
			if humanCode != code || !strings.Contains(human, "help:") {
				t.Fatal(humanCode, human)
			}
			for _, id := range []string{diagnostics.EscapeLocalStorage, diagnostics.EscapeOuterPlace, diagnostics.EscapeMatchPayload, diagnostics.EscapeClosureCapture, diagnostics.EscapeVariadicPack} {
				if !strings.Contains(human, id) {
					t.Fatal(id, human)
				}
			}
		})
	}
}
