package main

import (
	"sec/internal/diagnostics"
	"testing"
)

// TestCollectionGrowthCLI retains the shared transitive owning diagnostic in
// compilation and analysis, including navigation to the growth operation.
// Rules: rules/memory/allocation.md — §§24(4),28(4),29(1),(4).
func TestCollectionGrowthCLI(t *testing.T) {
	for _, command := range []string{"check", "analyse"} {
		_, stderr, code := runCLIForDiagnostics(t, command, "../../testdata/sema/collection_growth_invalid.sec", "--diagnostic-format=json")
		if code != 3 {
			t.Fatal(command, code, stderr)
		}
		doc := decodeOccurrenceDocument(t, stderr)
		if doc.Summary.Errors != 11 {
			t.Fatal(doc)
		}
		for _, occurrence := range doc.Occurrences {
			if occurrence.Severity != "error" {
				continue
			}
			if occurrence.ID == nil || *occurrence.ID != diagnostics.NoAllocViolation || len(occurrence.Related) == 0 {
				t.Fatal(occurrence)
			}
		}
	}
}
