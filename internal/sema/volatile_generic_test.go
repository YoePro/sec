package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Generic parameters do not acquire volatile eligibility from unsafe or an
// ordinary interface; ordinary pointer access and concrete volatile sites keep
// their separate requirements, including inside generic declarations.
// Rules: rules/platform/volatile.md — §§33,38;
// rules/memory/raw_pointers.md — §§11–13.
func TestVolatileGenericEligibility(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "volatile_generic_invalid.sec"
		if valid {
			name = "volatile_generic_valid.sec"
		}
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/" + name)
			if err != nil {
				t.Fatal(err)
			}
			a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
			want := 6
			if valid {
				want = 0
			}
			if len(errors) != want {
				t.Fatalf("got %d errors, want %d: %v", len(errors), want, errors)
			}
			lines := []int{6, 10, 14, 18, 25, 28}
			for index, err := range errors {
				if err.ID != diagnostics.RawPointerInvalidOperation || !strings.Contains(err.Message, "requires proven volatile eligibility for generic pointee T") || err.Line != lines[index] || err.Column == 0 {
					t.Fatalf("unexpected diagnostic: %+v", err)
				}
				if !strings.Contains(err.Help, "interface constraints") || !strings.Contains(err.Help, "unsafe") {
					t.Fatalf("missing distinction between eligibility and authorization: %+v", err)
				}
			}
			// A rejected operation must not publish a successful volatile event.
			if !valid {
				for _, node := range a.CallGraph().Nodes() {
					for _, effect := range a.CallGraph().EffectSummary(node.ID).DirectEffects {
						if effect.Kind == EffectVolatileRead || effect.Kind == EffectVolatileWrite {
							t.Fatalf("rejected operation published an effect: %+v", effect)
						}
					}
				}
			}
		})
	}
}
