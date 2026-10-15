package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Stable IDs classify semantic producers without replacing the existing
// mentor text or its origin/holder links. Escape keeps its established IDs
// except for the previously unregistered borrowed-Arena carrier restriction.
// Rules: rules/memory/borrowing.md — §§7,12,22,23,32–33;
// rules/tooling/diagnostics.md — §§5–7,15.
func TestBorrowDiagnosticIDs(t *testing.T) {
	for _, test := range []struct {
		file string
		ids  []string
	}{
		{"borrow_authority_invalid.sec", []string{"S1161", "S1162", "S1159", "S1160", "S1163", "S1163", "S1161"}},
		{"borrow_holder_transfer_invalid.sec", []string{"S1162", "S1164"}},
		{"borrow_overlap_mentor_invalid.sec", []string{"S1162", "S1161", "S1160", "S1159", "S1163"}},
		{"borrow_additional_ids_invalid.sec", []string{"S1165", "S1165", "S1167", "S1167", "S1128", "S1165", "S1163"}},
	} {
		t.Run(test.file, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/tooling/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			_, errors := analyzeSourceWithAnalyzer(t, string(data))
			if len(errors) != len(test.ids) {
				t.Fatalf("errors = %+v, want %v", errors, test.ids)
			}
			for index, issue := range errors {
				definition, registered := diagnostics.Lookup(issue.ID)
				if issue.ID != test.ids[index] || !registered || !definition.Mandatory || definition.DefaultSeverity != diagnostics.SeverityError || issue.Help == "" || issue.Line == 0 || issue.Column == 0 {
					t.Fatalf("diagnostic %d = %+v, want mandatory %s", index, issue, test.ids[index])
				}
			}
		})
	}
}

// An unknown-provenance rejection explains a missing proof and never fabricates
// a definitive borrow origin. Finite known alternatives remain a separate fact.
// Rules: rules/memory/borrowing.md — §§12(6–8),29,32.
func TestBorrowDiagnosticUnknownProvenanceID(t *testing.T) {
	a := NewAnalyzer()
	if !a.checkBorrowCreationPlace(Place{Root: "value", AmbiguousProvenance: true}, false, lexer.Token{File: "unknown.sec", Line: 3, Column: 4}) {
		t.Fatal("unknown provenance was accepted")
	}
	if len(a.errors) != 1 || a.errors[0].ID != "S1166" || a.errors[0].Help == "" || len(a.errors[0].RelatedLocations()) != 0 {
		t.Fatalf("unknown provenance diagnostic = %+v", a.errors)
	}
	definition, ok := diagnostics.Lookup(a.errors[0].ID)
	if !ok || !definition.Mandatory || definition.Name != "borrowing.provenance-unknown" {
		t.Fatalf("unregistered provenance category: %+v", definition)
	}
}
