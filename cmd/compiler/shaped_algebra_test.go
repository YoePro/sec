package main

import (
	"strings"
	"testing"
)

// TestShapedMultiplyExtentCLI rejects overflow in the derived shape before any
// lowering, retaining the contextual x source location in CLI diagnostics.
// Rules: rules/collections/shaped-types.md — §§4–5,22,29.
func TestShapedMultiplyExtentCLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/shaped_multiply_facts_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 || doc.Summary.Errors != 3 {
		t.Fatal(code, stderr)
	}
	found := false
	for _, item := range doc.Occurrences {
		if strings.Contains(item.Message.Text, "18446744073709551616 overflows target uint") {
			found = true
			if item.Primary == nil || item.Primary.Span.Start.Line != 9 {
				t.Fatal(item)
			}
		}
	}
	if !found {
		t.Fatal(stderr)
	}
}
