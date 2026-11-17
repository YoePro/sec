package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A concrete aggregate's ordinary layout is not an explicit physical access
// contract, even for empty structs, named wrappers or generic specializations.
// Ordinary pointer operations and user methods retain their separate semantics.
// Rules: rules/platform/volatile.md — §§33,37;
// rules/memory/raw_pointers.md — §§11–13.
func TestVolatileAggregateEligibility(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "volatile_aggregate_invalid.sec"
		if valid {
			name = "volatile_aggregate_valid.sec"
		}
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/" + name)
			if err != nil {
				t.Fatal(err)
			}
			a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
			want := 20
			if valid {
				want = 0
			}
			if len(errors) != want {
				t.Fatalf("got %d errors, want %d: %v", len(errors), want, errors)
			}
			for _, err := range errors {
				if err.ID != diagnostics.RawPointerInvalidOperation || !strings.Contains(err.Message, "requires an explicit physical layout and access contract for aggregate") || err.Line == 0 || err.Column == 0 {
					t.Fatalf("unexpected diagnostic: %+v", err)
				}
				if !strings.Contains(err.Help, "ordinary Sec layout") || !strings.Contains(err.Help, "unsafe") {
					t.Fatalf("missing physical-contract help: %+v", err)
				}
			}
			if !valid {
				for _, node := range a.CallGraph().Nodes() {
					for _, effect := range a.CallGraph().EffectSummary(node.ID).DirectEffects {
						if effect.Kind == EffectVolatileRead || effect.Kind == EffectVolatileWrite {
							t.Fatalf("rejected aggregate access published an effect: %+v", effect)
						}
					}
				}
			}
		})
	}
}
