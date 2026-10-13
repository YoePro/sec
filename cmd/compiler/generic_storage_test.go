package main

import (
	"strings"
	"testing"
)

// TestCheckGenericRecursiveStorageBoundaries verifies the semantic CLI accepts
// finite descriptors and retains mandatory rejection of inline layout cycles.
// Rules: rules/declarations/unions.md — §18;
// rules/memory/layout.md — §13(7), §25(1–5).
func TestCheckGenericRecursiveStorageBoundaries(t *testing.T) {
	for _, suffix := range []string{"valid", "invalid"} {
		t.Run(suffix, func(t *testing.T) {
			_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/generics/recursive_boundaries_"+suffix+".sec", "--diagnostic-format=json")
			if suffix == "valid" {
				if code != 0 {
					t.Fatal(code, stderr)
				}
				return
			}
			if code != 3 {
				t.Fatal(code, stderr)
			}
			doc := decodeOccurrenceDocument(t, stderr)
			if doc.Summary.Errors != 4 {
				t.Fatal(doc)
			}
			for _, value := range doc.Occurrences {
				if !strings.Contains(value.Message.Text, "infinite size") {
					t.Fatal(value)
				}
			}
		})
	}
}
