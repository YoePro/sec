package sema

import (
	"testing"

	"sec/internal/ast"
)

// P17-13: every use site that copies or moves a reusable value publishes a
// transfer plan: copies into return slots, constructed fields and payloads,
// arm results and closure environments; explicit moves into arm results;
// by-value payload, field, `if ... is` and try-handler bindings. Borrows and
// observations publish no transfer plan and classify separately.
//
// Rules:
//   - rules/memory/copy_move.md — §§ 3, 5–11
//   - rules/control-flow/flowcontrol_match.md — §§ 12–13, 21
//   - rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 16–18, 69
func TestTransferPlansCoverEveryUseSite(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

type Point struct {
    x: int,
    y: int,
}

type Figure union {
    Dot(int)
    Box {
        width: int,
        height: int,
    }
    Owned(Handle)
}

type LoadError union error {
    Busy(Handle)
    Stop
}

fn Load(flag: bool) Result[int, LoadError] {
    if flag {
        return Err(LoadError.Stop)
    }
    return Ok(1)
}

fn ReturnCopy(count: int) int {
    return count
}

fn ConstructCopy(left: int) Point {
    let point := Point { x: left, y: 2 }
    return point
}

fn PayloadCopy(value: int) Option[int] {
    let wrapped := Some(value)
    return wrapped
}

fn CaptureCopy(number: int) int {
    let closure := capture(number) fn() int { return number }
    return closure()
}

fn Bindings(shape: Figure) int {
    let total := match shape {
        Dot(size) => size
        Box { width: wide, height: tall } => wide + tall
        Owned(ref handle) => handle.id
    }
    return total
}

fn MoveBinding(shape: Figure) int {
    match shape {
        Owned(handle) => {
            discard handle
            return 1
        }
        _ => return 0
    }
}

fn OptionBinding(maybe: Option[int]) int {
    if maybe is Some(found) {
        return found
    }
    return 0
}

fn HandlerBinding(flag: bool) int {
    let value := try Load(flag) {
        Err(e) => 0
    }
    return value
}

fn BranchValues(maybe: Option[int], left: int, owned: Handle) Handle {
    let chosen := match maybe {
        Some(_) => left
        None => 2
    }
    let kept := match maybe {
        Some(_) => <-owned
        None => Handle { id: chosen }
    }
    return <-kept
}

fn Observe(point: Point) bool {
    let seen := ref point
    return point.x > 0
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
	plans := map[key]ResolvedTransferPlan{}
	for _, plan := range analyzer.ResolvedTransferPlans() {
		source := plan.Source.String()
		if plan.SourceFresh {
			source = "fresh " + typeDisplayName(plan.Source.Type)
		}
		plans[key{plan.Site, plan.Action, source}] = plan
	}
	for _, want := range []key{
		{TransferSiteReturn, TransferCopyTrivial, "count"},
		{TransferSiteConstruction, TransferCopyTrivial, "left"},
		{TransferSiteConstruction, TransferCopyTrivial, "value"},
		{TransferSiteCapture, TransferCopyTrivial, "number"},
		{TransferSiteBranch, TransferCopyTrivial, "left"},
		{TransferSiteBranch, TransferMove, "owned"},
		{TransferSiteMatchBinding, TransferMove, "fresh LoadError"},
	} {
		if _, ok := plans[want]; !ok {
			t.Errorf("missing transfer plan %+v", want)
		}
	}

	// Pattern bindings name the payload or field Place they take from and
	// their own local Place as destination.
	bindings := map[string]ResolvedTransferPlan{}
	for _, plan := range analyzer.ResolvedTransferPlans() {
		if plan.Site == TransferSiteMatchBinding && plan.Destination.Place != nil {
			bindings[plan.Destination.Place.Root] = plan
		}
	}
	for name, want := range map[string]TransferAction{"size": TransferCopyTrivial, "wide": TransferCopyTrivial, "tall": TransferCopyTrivial, "handle": TransferMove, "found": TransferCopyTrivial, "e": TransferMove} {
		plan, ok := bindings[name]
		if !ok {
			t.Errorf("binding %s has no transfer plan", name)
			continue
		}
		if plan.Action != want {
			t.Errorf("binding %s: action %s, want %s", name, plan.Action, want)
		}
		if name != "e" && (plan.SourceFresh || plan.Source.Root == "" || len(plan.Source.Projections) == 0) {
			t.Errorf("binding %s does not name its payload Place: %+v", name, plan.Source)
		}
	}
	if !bindings["handle"].Partial || bindings["handle"].OwnershipAfter != TransferMoved {
		t.Errorf("a moved payload binding is not a partial move of its subject: %+v", bindings["handle"])
	}

	// A borrow and an observation are not transfers.
	uses := map[OwnershipUseKind]int{}
	walkTransferTestNodes(analyzer, func(expr ast.Expression) {
		if use, ok := analyzer.ResolvedOwnershipUseOf(expr); ok {
			uses[use.Kind]++
			if use.Kind != OwnershipUseTransfer && use.Transfer != nil {
				t.Errorf("%s use carries a transfer plan", use.Kind)
			}
		}
	})
	if uses[OwnershipUseBorrowShared] == 0 || uses[OwnershipUseObservation] == 0 || uses[OwnershipUseTransfer] == 0 {
		t.Errorf("ownership uses do not distinguish transfer, borrow and observation: %v", uses)
	}
}

// walkTransferTestNodes visits every expression that has an analyzed Place,
// reference fact or transfer plan.
func walkTransferTestNodes(analyzer *Analyzer, visit func(ast.Expression)) {
	seen := map[ast.Expression]bool{}
	for expr := range analyzer.resolvedPlaces {
		seen[expr] = true
	}
	for expr := range analyzer.resolvedReferencePlans {
		seen[expr] = true
	}
	for expr := range analyzer.resolvedTransferPlans {
		seen[expr] = true
	}
	for expr := range seen {
		visit(expr)
	}
}
