package main

import (
	"strings"
	"testing"
)

// TestGenericImplConstraintsCLI verifies that check uses the owner's guarantees
// in both impl forms and diagnoses missing concrete or forwarded conjuncts.
// Rules: rules/declarations/generics.md — §§8,12,14–15,25.
func TestGenericImplConstraintsCLI(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/generic_impl_constraints_valid.sec", "--diagnostic-format=json"); code != 0 {
		t.Fatal(code, stderr)
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/generic_impl_constraints_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, stderr)
	}
	document := decodeOccurrenceDocument(t, stderr)
	if document.Summary.Errors != 8 {
		t.Fatal(stderr)
	}
	for _, diagnostic := range document.Occurrences {
		if !strings.Contains(diagnostic.Message.Text, "does not satisfy constraint") {
			t.Fatal(diagnostic)
		}
	}
}
