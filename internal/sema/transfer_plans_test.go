package sema

import (
	"testing"
)

// P17-12/13/14: every validated transfer from a reusable source Place
// publishes a detached plan with its site, action and source Place: copies of
// trivially copyable values, explicit moves of MoveOnly values in let,
// assignment, argument, return and construction position, and partial moves
// of a tracked local struct field.
//
// Rules:
//   - rules/memory/ownership.md — §§ 10–14
//   - rules/memory/copy_move.md — §§ 3, 5–10
func TestTransferPlansRecordSiteActionAndSource(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

type Pair struct {
    left: Handle,
    right: Handle,
}

fn Consume(value: Handle) void {
}

fn Make() Handle {
    return Handle { id: 1 }
}

fn Give() Handle {
    let owned := Make()
    return <-owned
}

fn Use() void {
    let count := 3
    let copy := count
    let first := Make()
    let second :<- first
    let mut target := Make()
    target <- second
    Consume(<-target)
    let pair := Pair { left: Make(), right: Make() }
    let piece :<- pair.left
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	type key struct {
		site   TransferSiteKind
		action TransferAction
		source string
	}
	seen := map[key]bool{}
	partial := false
	for _, plan := range analyzer.ResolvedTransferPlans() {
		seen[key{plan.Site, plan.Action, plan.Source.String()}] = true
		partial = partial || (plan.Partial && plan.Source.String() == "pair.left")
	}
	for _, want := range []key{
		{TransferSiteInitialization, TransferCopyTrivial, "count"},
		{TransferSiteInitialization, TransferMove, "first"},
		{TransferSiteAssignment, TransferMove, "second"},
		{TransferSiteArgument, TransferMove, "target"},
		{TransferSiteReturn, TransferMove, "owned"},
		{TransferSiteInitialization, TransferMove, "pair.left"},
	} {
		if !seen[want] {
			t.Errorf("missing transfer plan %+v in %+v", want, analyzer.ResolvedTransferPlans())
		}
	}
	if !partial {
		t.Error("field move is not marked partial")
	}
	plans := analyzer.ResolvedTransferPlans()
	if len(plans) == 0 {
		t.Fatal("no plans")
	}
	plans[0].Source.Root = "mutated"
	if again := analyzer.ResolvedTransferPlans(); again[0].Source.Root == "mutated" {
		t.Fatal("transfer plans alias analyzer state")
	}
}

// A rejected implicit copy of a MoveOnly value publishes no plan.
func TestRejectedTransferHasNoPlan(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

fn Use() void {
    let first := Handle { id: 1 }
    let second := first
}
`)
	if len(errors) == 0 {
		t.Fatal("implicit copy of a custom-free value was accepted")
	}
	for _, plan := range analyzer.ResolvedTransferPlans() {
		if plan.Source.Root == "first" {
			t.Fatalf("rejected transfer recorded a plan: %+v", plan)
		}
	}
}
