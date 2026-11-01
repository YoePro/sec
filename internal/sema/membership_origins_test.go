package sema

import (
	"os"
	"strings"
	"testing"
)

// A bool observation of local or arm-scoped borrowed storage does not retain
// that storage, including when stored or returned through a value aggregate.
// Actual reference carriers continue to obey ordinary escape restrictions.
// Rules: rules/foundations/operators.md — §A.14;
// rules/memory/lifetime_analysis.md — §§1–4; rules/memory/ownership.md — §28.1.
func TestMembershipDoesNotEscapeOperandReference(t *testing.T) {
	const base = "../../testdata/sema/membership_origins/"
	valid, err := os.ReadFile(base + "valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(valid)); len(errors) != 0 {
		t.Fatalf("bool membership retained operand borrows: %v", errors)
	}
	invalid, err := os.ReadFile(base + "escape_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(invalid))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot return reference to local variable") {
		t.Fatalf("lost actual escape rejection: %v", errors)
	}
}
