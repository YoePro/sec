package sema

import (
	"strings"
	"testing"
)

// Regression coverage for the logical short-circuit flow: a conditionally
// executed RHS merges its ownership action with the bypass path in every
// context and nesting, a while condition consumes on every iteration, and a
// reachable RHS call is published in the call graph.
//
// Rules:
//   - rules/foundations/operators.md — "Short-circuit evaluation"
//   - rules/control-flow/flowcontrol_if.md, flowcontrol_while.md
//   - rules/memory/ownership.md — conditional availability
func TestLogicalShortCircuitOwnershipAcrossContexts(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

@noCopy
type Token struct { id: int, }

fn Consume(-> token: Token) bool { return true }
fn Use(value: ref Token) void {}

fn Conditional(flag: bool) void {
    let token := Token { id: 1 }
    let result := flag && Consume(<-token)
    Use(ref token)
    discard result
}

fn Nested(flag: bool, other: bool) void {
    let token := Token { id: 2 }
    let result := flag && (other || Consume(<-token))
    Use(ref token)
    discard result
}

fn InIf(flag: bool) void {
    let token := Token { id: 3 }
    if flag || Consume(<-token) {
        Use(ref token)
    }
}

fn InWhile(flag: bool) void {
    let token := Token { id: 4 }
    while flag && Consume(<-token) {
    }
}

fn Folded() void {
    let token := Token { id: 5 }
    let result := 1 < 2 || Consume(<-token)
    Use(ref token)
    discard result
}
`)
	conditional := 0
	iteration := 0
	for _, err := range errs {
		switch {
		case strings.Contains(err.Message, "cannot borrow conditionally available place token"):
			conditional++
		case strings.Contains(err.Message, "place token may be unavailable on a later loop iteration"):
			iteration++
		default:
			t.Errorf("unexpected diagnostic %v", err)
		}
	}
	// Conditional, Nested and InIf borrow after a conditional RHS move; the
	// folded RHS of Folded never runs, so it keeps token available.
	if conditional != 3 || iteration != 1 {
		t.Fatalf("conditional=%d iteration=%d, want 3 and 1: %v", conditional, iteration, errs)
	}
}

// A while condition runs before every iteration, so a value it consumes is
// unavailable on the next one, with or without a logical operator.
// Rules: rules/control-flow/flowcontrol_while.md; rules/memory/ownership.md.
func TestWhileConditionMoveIsCheckedAcrossIterations(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

@noCopy
type Token struct { id: int, }

fn Consume(-> token: Token) bool { return true }

fn Plain() void {
    let token := Token { id: 1 }
    while Consume(<-token) {
    }
}

fn Reinitialized() void {
    let mut token := Token { id: 1 }
    while Consume(<-token) {
        token = Token { id: 2 }
    }
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "place token may be unavailable on a later loop iteration") {
		t.Fatalf("want only the Plain iteration error, got %v", errs)
	}
}

// A conditionally executed RHS call is a reachable call-graph edge with its
// effects, unlike a compiler-folded skipped RHS.
// Rules: rules/analysis/call_graph.md; rules/foundations/operators.md — "Short-circuit evaluation".
func TestLogicalConditionalRHSCallIsReachable(t *testing.T) {
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Dangerous() bool { panic "reachable logical RHS" }

fn Effects(flag: bool) bool {
    return flag || Dangerous()
}
`)
	assertSemaErrors(t, errs, nil)
	graph := analyzer.CallGraph()
	effectsID := callGraphNodeIDByName(t, graph, "Effects")
	if len(graph.Outgoing(effectsID)) != 1 {
		t.Fatalf("Effects outgoing calls = %+v, want the conditional RHS call", graph.Outgoing(effectsID))
	}
	if !graph.EffectSummary(effectsID).MayPanic {
		t.Fatalf("Effects must inherit the reachable RHS panic effect")
	}
}
