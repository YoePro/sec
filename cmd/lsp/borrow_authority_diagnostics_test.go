package main

import (
	"os"
	"strings"
	"testing"
)

// TestBorrowAuthorityDiagnosticsLSP verifies that published diagnostics retain
// Sema's availability/authority explanation and navigable borrow origin.
// Rules: rules/memory/borrowing.md — §32(7–8),§33(1).
func TestBorrowAuthorityDiagnosticsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/borrow_authority_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	values := analyze("file:///tmp/sec-borrow-authority/main.sec", string(data))
	if len(values) != 7 {
		t.Fatalf("diagnostics: %+v", values)
	}
	for _, value := range values {
		if value.Severity != 1 || !strings.Contains(value.Message, "is available (still owned), but an active borrow blocks this access") || !strings.Contains(value.Message, "not borrow access authority") || len(value.RelatedInformation) != 2 || value.RelatedInformation[0].Location.Range.Start.Line != value.Range.Start.Line-1 {
			t.Fatalf("lost compiler explanation or borrow origin: %+v", value)
		}
	}
}
