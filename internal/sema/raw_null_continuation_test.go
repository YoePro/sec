package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
)

// TestRawNullContinuation checks pure compound false-edge proofs, including
// early exits, else branches, copied values and both safe-reference properties.
// A false conjunction or true disjunction does not identify which operand held.
// Rules: rules/memory/raw_pointers.md §§7(3–8),18(5),34(2–4);
// rules/memory/unsafe.md §§21(2),22(4).
func TestRawNullContinuation(t *testing.T) {
	for _, tc := range []struct {
		name string
		want int
	}{
		{"invalid", 6}, {"valid", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/raw_null_continuation_" + tc.name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			errors := analyzeSource(t, string(source))
			if len(errors) != tc.want {
				t.Fatalf("errors = %v, want %d null errors", errors, tc.want)
			}
			for _, e := range errors {
				if e.ID != diagnostics.RawPointerNullDereference {
					t.Fatalf("unexpected error: %v", e)
				}
			}
		})
	}
}
