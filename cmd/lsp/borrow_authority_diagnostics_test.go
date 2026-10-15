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
	ids := []string{"S1161", "S1162", "S1159", "S1160", "S1163", "S1163", "S1161"}
	for index, value := range values {
		if value.Code != ids[index] {
			t.Fatalf("lost stable borrow ID: %+v", value)
		}
		if value.Severity != 1 || !strings.Contains(value.Message, "is available (still owned), but an active borrow blocks this access") || !strings.Contains(value.Message, "not borrow access authority") || len(value.RelatedInformation) != 2 || value.RelatedInformation[0].Location.Range.Start.Line != value.Range.Start.Line-1 {
			t.Fatalf("lost compiler explanation or borrow origin: %+v", value)
		}
	}
}

// The language server projects compiler IDs without a second borrow checker,
// including loop-carried state and previously unregistered Arena escape.
// Rules: rules/memory/borrowing.md — §§7,23,32–33;
// rules/tooling/diagnostics.md — §15.
func TestAdditionalBorrowDiagnosticIDsLSP(t *testing.T) {
	for _, test := range []struct {
		file string
		ids  []string
	}{
		{"borrow_additional_ids_invalid.sec", []string{"S1165", "S1165", "S1167", "S1167", "S1128", "S1165", "S1163"}},
		{"borrow_holder_transfer_invalid.sec", []string{"S1162", "S1164"}},
		{"borrow_overlap_mentor_invalid.sec", []string{"S1162", "S1161", "S1160", "S1159", "S1163"}},
	} {
		data, err := os.ReadFile("../../testdata/tooling/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-borrow-ids/main.sec", string(data))
		if len(values) != len(test.ids) {
			t.Fatal(test.file, values)
		}
		for index, value := range values {
			if value.Code != test.ids[index] || value.Severity != 1 || !strings.Contains(value.Message, "help:") {
				t.Fatalf("%s: %+v", test.file, value)
			}
		}
	}
}
