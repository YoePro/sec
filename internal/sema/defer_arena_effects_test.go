package sema

import (
	"os"
	"reflect"
	"testing"
)

// Future invalidations retain the canonical body sequence and receiver rather
// than leaking immediate calls or nested callable bodies into deferred cleanup.
// Rules: rules/control-flow/defer.md — §§3,6,9,29;
// rules/memory/arena.md — §§35,43,52,76–77.
func TestDeferredArenaInvalidationPlans(t *testing.T) {
	a, program := analyzeDeferPlanFixture(t, "defer_arena_effects.sec")
	before := map[DeferID]ResolvedDeferPlan{}
	for statement := range a.resolvedDeferPlans {
		plan, _ := a.ResolvedDeferPlanOf(statement)
		before[plan.Identity.Defer] = cloneResolvedDeferPlan(plan)
		name := a.callGraph.nodes[plan.Invocation].Name
		switch name {
		case "Ordered":
			want := []ArenaOperationKind{ArenaOperationReset, ArenaOperationReset, ArenaOperationRelease}
			if len(plan.ArenaInvalidations) != len(want) {
				t.Fatalf("deferred sites = %+v, want %v", plan.ArenaInvalidations, want)
			}
			for index, effect := range plan.ArenaInvalidations {
				if effect.Operation != want[index] || effect.Receiver.RootID == 0 || effect.Receiver.Root != "mem" {
					t.Fatalf("invalid receiver/operation: %+v", effect)
				}
				if index > 0 && effect.DirectEffectIndex <= plan.ArenaInvalidations[index-1].DirectEffectIndex {
					t.Fatal("deferred operation order lost")
				}
				event := a.callGraph.arenaEffects[plan.Body][effect.DirectEffectIndex]
				if event.Source != effect.Source || string(event.Kind) != string(effect.Operation) {
					t.Fatal("future effect no longer identifies its canonical body event")
				}
			}
			// Returned Places must not alias the stored plan.
			plan.ArenaInvalidations[0].Receiver.Root = "changed"
			plan.ArenaInvalidations[0].Receiver.Type.Name = "changed"
			plan.ArenaInvalidations[0].Operation = ArenaOperationAlloc
			again, _ := a.ResolvedDeferPlanOf(statement)
			if !reflect.DeepEqual(again, before[plan.Identity.Defer]) {
				t.Fatal("query mutates stored future effects")
			}
		case "ImmediateOnly", "NestedCallable":
			if len(plan.ArenaInvalidations) != 0 {
				t.Fatalf("%s contains another callable's effects: %+v", name, plan.ArenaInvalidations)
			}
		default:
			t.Fatalf("unexpected invocation %s", name)
		}
	}
	if len(before) != 3 {
		t.Fatalf("want three defer plans, got %d", len(before))
	}
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for statement := range a.resolvedDeferPlans {
		plan, _ := a.ResolvedDeferPlanOf(statement)
		if !reflect.DeepEqual(plan, before[plan.Identity.Defer]) {
			t.Fatal("reanalysis changed future invalidation facts")
		}
	}
}

// The published future effects must coexist with unchanged ordinary
// continuation validity; invalid deferred bodies publish no consumable plan.
// Rules: rules/control-flow/defer.md — §§9,29;
// rules/memory/arena.md — §§35–36,43–44,52.
func TestDeferredArenaInvalidationContinuationAndRejection(t *testing.T) {
	for _, name := range []string{"valid", "body_invalid"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/control_flow/defer_arena_continuation/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
			if name == "body_invalid" {
				if len(errors) != 1 || len(a.resolvedDeferPlans) != 0 {
					t.Fatalf("rejected deferred body published effects: errors=%v plans=%v", errors, a.resolvedDeferPlans)
				}
				return
			}
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			if len(a.resolvedDeferPlans) != 8 {
				t.Fatalf("want eight deferred sites, got %d", len(a.resolvedDeferPlans))
			}
			for statement := range a.resolvedDeferPlans {
				plan, _ := a.ResolvedDeferPlanOf(statement)
				if len(plan.ArenaInvalidations) != 1 || plan.Identity.Action == 0 {
					t.Fatalf("missing future effect or registration identity: %+v", plan)
				}
			}
		})
	}
}
