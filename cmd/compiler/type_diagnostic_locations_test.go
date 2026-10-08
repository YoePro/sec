package main

import (
	"strings"
	"testing"
)

// TestTypeDiagnosticLocationsCLI retains owning IDs and cross-file defining
// locations in machine-readable and human diagnostics through the shared CLI diagnostic reporter.
// Rules: rules/types/contracts.md — Diagnostics; rules/types/default_values.md — Diagnostics;
// rules/tooling/diagnostics.md — §§12,14.
func TestTypeDiagnosticLocationsCLI(t *testing.T) {
	const base = "../../testdata/sema/type_locations/definitions.sec"
	const use = "../../testdata/sema/type_locations/use_invalid.sec"
	for _, command := range []string{"sema", "analyse"} {
		t.Run(command, func(t *testing.T) {
			args := []string{command, "../../testdata/sema/type_locations"}
			_, output, code := runCLIForDiagnostics(t, append(args, "--diagnostic-format=json")...)
			document := decodeOccurrenceDocument(t, output)
			if code != 3 || document.Summary.Errors != 13 || len(document.Occurrences) != 13 {
				t.Fatal(code, output)
			}
			for _, e := range document.Occurrences {
				if e.ID == nil || len(e.Related) != 1 || e.Related[0].Message == nil || e.Related[0].Span.Start.Line == 0 {
					t.Fatal(e)
				}
				if e.Primary.Span.File == use && e.Related[0].Span.File != base && e.Related[0].Span.File != use {
					t.Fatal("lost defining file", e)
				}
			}
			_, human, humanCode := runCLIForDiagnostics(t, args...)
			if humanCode != code || !strings.Contains(human, "contract declaration at") || !strings.Contains(human, "invalid explicit default at") || !strings.Contains(human, "field requiring initialization at") {
				t.Fatal(humanCode, human)
			}
		})
	}
}
