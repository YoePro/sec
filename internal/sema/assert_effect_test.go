package sema

import (
	"os"
	"strings"
	"testing"
)

// rules/errors/panic.md §§15.3, 15.5, and 15.8 require a reachable, unproven
// assertion to contribute MayPanic while a proven-true assertion does not.
func TestAssertionPanicEffectsAndTransitivePath(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/assert_effects_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, nil)
	graph := analyzer.CallGraph()
	for _, name := range []string{"Proven", "DeadBranch"} {
		id := callGraphNodeIDByName(t, graph, name)
		if proven := graph.EffectSummary(id); proven.MayPanic || len(proven.DirectEffects) != 0 {
			t.Fatalf("%s assertion effects = %+v, want panic-free", name, proven)
		}
	}
	for _, name := range []string{"MayFail", "AlwaysFails"} {
		id := callGraphNodeIDByName(t, graph, name)
		summary := graph.EffectSummary(id)
		if !summary.MayPanic || len(summary.DirectEffects) != 1 || summary.DirectEffects[0].Kind != EffectMayPanicAssertion || summary.DirectEffects[0].Source.Lexeme != "assert" {
			t.Fatalf("%s effects = %+v, want one direct assertion-panic site", name, summary)
		}
	}
	mayFailID := callGraphNodeIDByName(t, graph, "MayFail")
	wrap := graph.EffectSummary(callGraphNodeIDByName(t, graph, "Wrap"))
	if !wrap.MayPanic || len(wrap.DirectEffects) != 0 || len(wrap.PanicPath) != 2 || wrap.PanicPath[1] != mayFailID {
		t.Fatalf("Wrap effects = %+v, want transitive MayFail assertion path", wrap)
	}
}

func TestUnprovenAssertionViolatesNoPanicDirectlyAndTransitively(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/assert_effects_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 2 {
		t.Fatalf("errors = %+v, want two @noPanic violations", errors)
	}
	for index, name := range []string{"Reject", "RejectTransitively"} {
		message := errors[index].Message
		if !strings.Contains(message, "function "+name+" does not satisfy @noPanic") ||
			!strings.Contains(message, string(EffectMayPanicAssertion)) || !strings.Contains(message, "effect introduced at") {
			t.Fatalf("error %d = %q, want cause-aware assertion violation for %s", index, message, name)
		}
	}
}
