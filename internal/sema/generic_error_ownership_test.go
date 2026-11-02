package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// MD-120's error category establishes neither copyability nor discardability;
// explicit consuming transfers still retain an exact owned channel. Source
// conformance does not imply executable per-specialization cleanup support.
// Rules: rules/declarations/generics.md — §15A;
// rules/errors/errorhandling.md — §§20,29; MD-120 §§4(4–7),11(4).
func TestGenericErrorOwnership(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_error_ownership/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	owned := a.Types()["OwnedError"]
	if !MoveOnly(owned) || TriviallyDestructible(owned) {
		t.Fatal("fixture lost nontrivial owned error traits")
	}
	sources := map[string]bool{"cause": false, "fallback": false}
	concrete := false
	for _, p := range a.ResolvedTransferPlans() {
		classification := CopyClassificationOf(p.Source.Type)
		if classification == CopyMoveOnly || classification == CopyConditional {
			if p.Action != TransferMove || p.OwnershipAfter != TransferMoved {
				t.Fatalf("owner was copied or retained: %+v", p)
			}
		}
		if p.Source.Type.Kind == GenericType && p.Source.Type.ErrorAssignable {
			if _, wanted := sources[p.Source.Root]; wanted {
				sources[p.Source.Root] = true
			}
		}
		concrete = concrete || p.Source.Type.Name == "OwnedError" && p.Action == TransferMove
	}
	for name, found := range sources {
		if !found {
			t.Errorf("missing generic owned transfer for %s", name)
		}
	}
	if !concrete {
		t.Error("missing concrete move-only E transfer")
	}
	ownedBindings := 0
	for expr := range a.resolvedMatchPlans {
		plan, ok := a.ResolvedMatchPlanOf(expr)
		if !ok {
			continue
		}
		for _, arm := range plan.Arms {
			if arm.BindingType.Kind != GenericType || !arm.BindingType.ErrorAssignable {
				continue
			}
			ownedBindings++
			if arm.BindingAction != MatchBindingMove && arm.BindingAction != MatchBindingConditional {
				t.Fatalf("unknown E match binding silently copied: %+v", arm)
			}
		}
	}
	if ownedBindings == 0 {
		t.Fatal("missing owned whole-error match bindings")
	}
	widened, guarded := false, false
	for expr := range a.resolvedTries {
		fact, ok := a.ResolvedTryOf(expr)
		if ok && fact.ErrorType.Kind == GenericType && len(fact.EnclosingResultType.TypeArgs) == 2 && fact.EnclosingResultType.TypeArgs[1].Kind == ErrorRootType {
			widened = true
		}
		plan, ok := a.ResolvedTryPlanOf(expr)
		if !ok || len(plan.Handlers) != 2 || !plan.Handlers[0].Guarded {
			continue
		}
		guarded = true
		if !plan.Exhaustive || plan.ResidualPropagates {
			t.Fatal("guarded whole-channel fallback lost exhaustiveness")
		}
		for _, h := range plan.Handlers {
			if h.BindingCopies || h.BindingType.Kind != GenericType || !h.BindingType.ErrorAssignable {
				t.Fatalf("unknown E binding copied or erased: %+v", h)
			}
		}
	}
	if !widened || !guarded {
		t.Fatalf("missing exact-to-open propagation or guarded owned binding facts: widening=%v guarded=%v", widened, guarded)
	}
}

// No specialization can retroactively authorize template copies/discards, and
// guard-false continuations keep their prospective error ownership intact.
// Rules: rules/errors/errorhandling.md — §§17,20,29; MD-120 §§4(4–7),11(4).
func TestGenericErrorOwnershipRejectsInvalidTransfers(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_error_ownership/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	counts := map[string]int{}
	for _, e := range errors {
		counts[e.ID]++
		if e.Severity != diagnostics.SeverityError {
			t.Fatalf("ownership rejection is not an error: %+v", e)
		}
		if e.ID == diagnostics.GuardConsumesBinding && e.Help == "" {
			t.Fatalf("guard rejection lacks repair: %+v", e)
		}
	}
	if len(errors) != 11 || counts[diagnostics.ImplicitMoveDisallowed] != 4 || counts[diagnostics.UseAfterMove] != 2 || counts[diagnostics.NonDiscardableValue] != 3 || counts[diagnostics.GuardConsumesBinding] != 2 {
		t.Fatalf("invalid ownership diagnostics: %+v", errors)
	}
}
