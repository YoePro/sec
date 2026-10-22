package sema

import (
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// P17-12: every currently produced transfer has an owning continuation and a
// source transition. Places and selected argument/return slots are authoritative
// facts, not names or signatures reconstructed by a later builder.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§9–11,17–18,60,67–68;
// rules/memory/copy_move.md — §§5,8–12.
func TestTransferDestinationAndOwnershipFacts(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "transfer_destinations.sec")
	seen := map[TransferSiteKind]bool{}
	ids := map[TransferDestinationID]bool{}
	for _, plan := range a.ResolvedTransferPlans() {
		if plan.Destination.ID == 0 || plan.Destination.Site != plan.Site || plan.Destination.Context == "" || ids[plan.Destination.ID] {
			t.Fatalf("missing or conflated destination identity: %+v", plan)
		}
		ids[plan.Destination.ID] = true
		seen[plan.Site] = true
		if plan.OwnershipBefore != TransferInitialized || plan.Action == TransferMove && plan.OwnershipAfter != TransferMoved ||
			plan.Action != TransferMove && plan.OwnershipAfter != TransferInitialized {
			t.Fatalf("incorrect source state transition: %+v", plan)
		}
		switch plan.Site {
		case TransferSiteInitialization, TransferSiteAssignment:
			if plan.Destination.Place == nil || plan.Destination.Place.RootID == 0 {
				t.Fatalf("local transfer lost its destination Place: %+v", plan)
			}
		case TransferSiteArgument:
			if plan.Destination.Callable == "" || plan.Destination.Parameter.Line <= 0 || plan.Destination.ArgumentIndex != 0 {
				t.Fatal("argument lost selected callable/parameter position")
			}
		case TransferSiteReturn:
			if plan.Destination.Callable != plan.Destination.Context || plan.Destination.Place != nil {
				t.Fatal("return slot became local source storage")
			}
		}
		want := map[string]string{"source": "initialized", "replacement": "target", "fieldValue": "pair.left", "pair.right": "part"}
		if destination, ok := want[plan.Source.String()]; ok && (plan.Destination.Place == nil || plan.Destination.Place.String() != destination) {
			t.Errorf("%s destination = %+v, want %s", plan.Source.String(), plan.Destination, destination)
		}
		if plan.Source.String() == "pair.right" && (!plan.Partial || len(plan.Source.Projections) != 1) {
			t.Fatal("partial transfer path was replaced by its owner root")
		}
	}
	for _, site := range []TransferSiteKind{TransferSiteInitialization, TransferSiteAssignment, TransferSiteArgument, TransferSiteReturn, TransferSiteConstruction, TransferSiteCapture} {
		if !seen[site] {
			t.Errorf("missing destination kind %s", site)
		}
	}
}

// Both query APIs detach destination Places/types as well as source facts.
// Repeated queries/reanalysis preserve identities and never alter ownership.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §18.
func TestTransferDestinationQueriesDetachedAndStable(t *testing.T) {
	a, program := analyzeDeferPlanFixture(t, "transfer_destinations.sec")
	reference, _ := analyzeDeferPlanFixture(t, "transfer_destinations.sec")
	want := reference.ResolvedTransferPlans()
	beforeMoves, beforeBorrows := copyMoved(a.moved), copyBorrows(a.borrows)
	beforeTypes, beforeIDs := len(a.expressionTypes), len(a.transferDestinations)
	mutate := func(plan *ResolvedTransferPlan) {
		plan.Destination.ID = 0
		if place := plan.Destination.Place; place != nil {
			place.Root = "changed"
			if len(place.Type.Fields) != 0 {
				place.Type.Fields[0].Name = "changed"
			}
			if len(place.Projections) != 0 {
				place.Projections[0].Name = "changed"
			}
		}
	}
	for value := range a.resolvedTransferPlans {
		plan, ok := a.ResolvedTransferPlanOf(value)
		if !ok {
			t.Fatal("missing source-use query")
		}
		mutate(&plan)
	}
	plans := a.ResolvedTransferPlans()
	for index := range plans {
		mutate(&plans[index])
	}
	if !reflect.DeepEqual(want, a.ResolvedTransferPlans()) || !reflect.DeepEqual(beforeMoves, a.moved) || !reflect.DeepEqual(beforeBorrows, a.borrows) ||
		beforeTypes != len(a.expressionTypes) || beforeIDs != len(a.transferDestinations) {
		t.Fatal("destination queries changed analysis facts or allocated identities")
	}
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if !reflect.DeepEqual(want, a.ResolvedTransferPlans()) {
		t.Fatal("destination identity changed on reanalysis")
	}
}

// Distinct synthesized use nodes can share token coordinates. They must not
// share an owning continuation, while revisiting the same semantic use is stable.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§9,17–18.
func TestTransferDestinationSeparatesSameLocationUses(t *testing.T) {
	a := NewAnalyzer()
	token := lexer.Token{File: "generated.sec", Line: 1, Column: 1}
	first := &ast.Identifier{Token: token, Value: "value"}
	second := &ast.Identifier{Token: token, Value: "value"}
	one := a.transferDestination(first, TransferSiteConstruction)
	two := a.transferDestination(second, TransferSiteConstruction)
	if one.ID == 0 || one.ID == two.ID || a.transferDestination(first, TransferSiteConstruction) != one {
		t.Fatal("source coordinates were used as owning destination identity")
	}
	place := Place{Root: "value", RootID: 1, Type: Type{Name: "int", Kind: IntType}}
	a.recordTransferPlan(first, place, TransferSiteConstruction, TransferCopyTrivial)
	a.recordTransferPlan(second, place, TransferSiteConstruction, TransferCopyTrivial)
	for attempt := 0; attempt < 10; attempt++ {
		plans := a.ResolvedTransferPlans()
		if len(plans) != 2 || plans[0].Destination.ID != one.ID || plans[1].Destination.ID != two.ID {
			t.Fatal("equal-position transfer queries have unstable ordering")
		}
	}
}
