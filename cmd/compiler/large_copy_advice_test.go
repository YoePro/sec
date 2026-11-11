package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Copy-cost guidance remains optional design advice in check and analyse;
// source validity, ownership diagnostics and contracts remain unchanged.
// Rules: rules/memory/copy_move.md §22(1–6);
// rules/analysis/parameter_usage_analysis.md §§63–65,92.
func TestLargeCopyAdviceCLI(t *testing.T) {
	for _, command := range [][]string{{"check"}, {"analyse", "parameter-usage"}} {
		args := append(append([]string{}, command...), "../../testdata/tooling/large_copy_advice.sec", "--diagnostic-format=json")
		_, output, code := runCLIForDiagnostics(t, args...)
		if code != 0 {
			t.Fatalf("legal copies rejected: code %d: %s", code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Errors != 0 {
			t.Fatal(output)
		}
		count := 0
		for _, d := range doc.Occurrences {
			if d.ID == nil || *d.ID != diagnostics.LargeValueParameter {
				continue
			}
			count++
			if len(d.Fixes) != 0 || len(d.Help) != 1 || !strings.Contains(d.Help[0].Text, "Estimated copy size:") || !strings.Contains(d.Help[0].Text, "by-value copy is legal") || !strings.Contains(d.Help[0].Text, "not an ownership repair") || strings.Contains(d.Message.Text, "Owner") {
				t.Fatalf("copy advice lost its evidence/optional status: %+v", d)
			}
		}
		if count != 2 {
			t.Fatalf("want Read/Forward advice only, got %d: %s", count, output)
		}
	}
}
