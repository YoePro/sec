package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Nonempty literal syntax builds fixed arrays, never owning dynamic arrays.
// Context applies equally to declarations, assignments, returns, aggregate
// fields; rejection must not publish a literal lowering plan.
// Rules: rules/collections/collections.md §§5.5–5.6a; rules/types/types.md
// "Contextual None and empty-literal arguments" (MD-027).
func TestDynamicArrayLiteralsRejectNonemptyOwners(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"declaration", "fn F() void { let values: int[] := [1, 2] }"},
		{"spread", "fn F() void { let fixed := [1, 2]\n let values: int[] := [fixed...] }"},
		{"assignment", "fn F() void { let mut values: int[]\n values = [1] }"},
		{"return", "fn F() int[] { return [1] }"},
		{"field", "type Box struct { values: int[] }\nfn F() void { let box := Box { values: [1] } }"},

		{"nested", "fn F() void { let values: int[1][] := [[1]] }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			analyzer, errors := analyzeSourceWithAnalyzer(t, "module main\n"+test.source)
			found := false
			for _, err := range errors {
				if err.ID == diagnostics.DynamicArrayLiteralInvalid {
					found = true
					if err.Line == 0 || err.Column == 0 || !strings.Contains(err.Help, "Append") || !strings.Contains(err.Message, "owning dynamic array") {
						t.Fatalf("incomplete diagnostic: %+v", err)
					}
				}
			}
			if !found {
				t.Fatalf("errors = %v, want S1155", errors)
			}
			wantPlans := 0
			if test.name == "spread" {
				wantPlans = 1 // The separately declared fixed source remains valid.
			}
			if len(analyzer.resolvedArrayLiteralPlans) != wantPlans {
				t.Fatalf("literal plans = %d, want %d", len(analyzer.resolvedArrayLiteralPlans), wantPlans)
			}
		})
	}
}

// An independently typed nonempty argument is a fixed array; ordinary calls
// must retain the fixed-versus-owner incompatibility rather than convert it.
// Rules: rules/collections/collections.md §§5.5,6.1,11.
func TestDynamicArrayLiteralsRejectImplicitArgumentConversion(t *testing.T) {
	_, errors := analyzeSourceWithAnalyzer(t, "module main\nfn Take(values: int[]) void {}\nfn F() void { Take([1]) }")
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "argument 1 to Take must be int[], got int[1]") {
		t.Fatalf("errors = %v, want fixed/owner argument mismatch", errors)
	}
}

// MD-027 permits contextual empty literals independently of the restriction
// on nonempty owners. Empty defaults do not require a defaultable element.
// Rules: rules/types/types.md "Contextual None and empty-literal arguments";
// rules/collections/collections.md §§5.5,6.2,6.7.
func TestDynamicArrayLiteralsKeepEmptyAndFixedForms(t *testing.T) {
	_, errors := analyzeSourceWithAnalyzer(t, `module main
type NoDefault int range 1..9
type Box struct { values: int[] }
fn Take(values: int[]) void {}
fn Empty() int[] { return [] }
fn F() Result[void, CollectionError] {
    let mut values: int[]
    values = []
    try values.Append(1)
    let empty: NoDefault[] := []
    Take([])
    let box := Box { values: [] }
    let fixed: int[3] := [1..3]
    let inferred := [1, 2]
    let spread: int[2] := [inferred...]
    return Ok()
}
`)
	if len(errors) != 0 {
		t.Fatalf("valid empty/fixed forms rejected: %v", errors)
	}
}
