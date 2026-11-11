package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
)

// Nested borrowed Arenas preserve the parent allocation's validity dependency
// until their own domain ends, without reclaiming parent storage individually.
// Rules: rules/memory/arena.md §§10,36,43–44,51(1–7).
func TestNestedArenaDependencies(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_nested_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 8 {
		t.Fatalf("want seven invalidations and one escape error: %v", errors)
	}
	for i, e := range errors {
		want := diagnostics.ArenaReleaseLiveDependency
		if i < 2 || i == 4 || i == 6 {
			want = diagnostics.ArenaResetLiveDependency
		}
		if i == 7 {
			want = diagnostics.BorrowOwnershipEndConflict
		}
		if e.ID != want {
			t.Fatalf("unexpected dependency error: %+v", e)
		}
		if e.PreviousLine == 0 || e.Help == "" || i < 7 && e.ProofState != diagnostics.ProofInvalid {
			t.Fatalf("missing dependency provenance: %+v", e)
		}
	}
	valid, err := os.ReadFile("../../testdata/sema/arena_nested_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(valid)), nil)
}
