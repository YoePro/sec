package sema

import (
	"testing"

	"sec/internal/diagnostics"
)

// rules/types/contracts.md — compile-time literal proofs apply at parameter
// and return boundaries as well as storage initialization.
func TestContractedIntegerLiteralsAreCheckedAtCallAndReturnBoundaries(t *testing.T) {
	errors := analyzeSource(t, `
type Percent int range 0..100

fn Accept(value: Percent) void {}

fn Call() void {
    Accept(101)
}

fn InvalidReturn() Percent {
    return 101
}
`)
	if len(errors) != 2 {
		t.Fatalf("contract boundary errors = %v, want 2", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.Message != "value 101 violates range contract Percent 0..100" {
			t.Fatalf("contract boundary diagnostic = %q", diagnostic.Message)
		}
	}
}

// Duplicate in-contract entries use one stable type diagnostic and relate the
// rejected value to its first source-ordered occurrence.
//
// Rules:
//   - rules/types/contracts.md — "Ordered membership" and "Diagnostics"
//   - rules/types/default_values.md — "Diagnostics"
//   - rules/tooling/diagnostics.md — § 2(6)–(8), § 3(4)–(6), and §§ 5–7
func TestDuplicateContractMembershipValueHasStableDiagnostic(t *testing.T) {
	errors := analyzeSource(t, `
type Role string in ["admin", "user", "admin"]
`)
	if len(errors) != 1 {
		t.Fatalf("duplicate membership errors = %+v, want 1", errors)
	}
	diagnostic := errors[0]
	if diagnostic.ID != diagnostics.DuplicateContractMembershipValue ||
		diagnostic.Severity != diagnostics.SeverityError ||
		diagnostic.Help != "remove the duplicate or replace it with a distinct permitted value" {
		t.Fatalf("duplicate membership diagnostic = %+v", diagnostic)
	}
	if diagnostic.PreviousLine == 0 || diagnostic.PreviousColumn == 0 || diagnostic.PreviousColumn >= diagnostic.Column {
		t.Fatalf("duplicate membership related location = %+v", diagnostic)
	}
}

// An empty in-contract is a recognized invalid type declaration rather than a
// generic syntax failure, and therefore carries its canonical type identity.
//
// Rules:
//   - rules/types/contracts.md — "Ordered membership" and "Diagnostics"
//   - rules/types/default_values.md — "Empty in [...] list" and "Diagnostics"
func TestEmptyContractMembershipHasStableDiagnostic(t *testing.T) {
	errors := analyzeSource(t, `
type Impossible int in []
`)
	if len(errors) != 1 {
		t.Fatalf("empty membership errors = %+v, want 1", errors)
	}
	diagnostic := errors[0]
	if diagnostic.ID != diagnostics.EmptyContractMembership ||
		diagnostic.Severity != diagnostics.SeverityError ||
		diagnostic.Help != "add at least one permitted compile-time value to the in-contract" ||
		diagnostic.EndColumn <= diagnostic.Column {
		t.Fatalf("empty membership diagnostic = %+v", diagnostic)
	}
}
