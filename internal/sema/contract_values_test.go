package sema

import "testing"

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
