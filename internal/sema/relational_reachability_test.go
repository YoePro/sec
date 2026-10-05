package sema

import (
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// A branch whose integer condition the dominating facts of its path decide is
// proven unreachable and reported as S3001: an enclosing branch, an else
// branch, the code after an exiting branch, and else-if exhaustion. Bindings
// that a loop, an assignment, a mutable borrow, or a lambda may change, and
// member reads, never produce such a proof.
//
// Rules:
//   - rules/analysis/call_graph.md — "Unreachable code"
//   - rules/tooling/diagnostics.md — § 21 "Proven unreachable and dead code"
//   - rules/control-flow/flowcontrol_if.md — § 20
func TestDominatingRelationsProveBranchesUnreachable(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "contradicting nested condition", want: 1, body: `
fn F(value: int) int {
    if value > 10 {
        if value < 5 {
            return 1
        }
    }
    return 0
}`},
		{name: "repeated after exit", want: 1, body: `
fn F(value: int) int {
    if value > 10 {
        return 1
    }
    if value > 20 {
        return 2
    }
    return 0
}`},
		{name: "else if exhaustion", want: 1, body: `
fn F(value: int) int {
    if value > 0 {
        return 1
    } else if value <= 0 {
        return 2
    }
    return 3
}`},
		{name: "always true leaves else impossible", want: 1, body: `
fn F(low: int, high: int) int {
    if low < high {
        if low <= high {
            return 1
        } else {
            return 2
        }
    }
    return 0
}`},
		{name: "consistent nested condition", body: `
fn F(value: int) int {
    if value > 10 {
        if value < 50 {
            return 1
        }
    }
    return 0
}`},
		{name: "loop changes the binding", body: `
fn F(x: int, n: int) int {
    let mut v := x
    if v > 10 {
        return 0
    }
    let mut i := 0
    while i < n {
        if v > 10 {
            return 1
        }
        v += 5
        i += 1
    }
    return 2
}`},
		{name: "assignment changes the parameter", body: `
fn F(value: int) int {
    if value > 10 {
        return 0
    }
    value = value + 20
    if value > 10 {
        return 1
    }
    return 2
}`},
		{name: "mutable borrow changes the binding", body: `
fn Bump(target: ref mut int) void {}

fn F(value: int) int {
    let mut current := value
    if current > 10 {
        return 0
    }
    Bump(ref mut current)
    if current > 10 {
        return 1
    }
    return 2
}`},
		{name: "type range alone is not a path proof", body: `
fn F(count: uint, flag: int) int {
    if flag > 0 {
        if count < 0 {
            return 1
        }
    }
    return 0
}`},
		{name: "member reads are not stable", body: `
fn F(values: ref list[int]) int {
    if values.Len > 10 {
        if values.Len < 5 {
            return 1
        }
    }
    return 0
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := parser.New(lexer.New("module main\n" + test.body + "\n")).ParseProgram()
			analyzer := NewAnalyzer()
			errors := analyzer.Analyze(program)
			unreachable := 0
			for _, err := range errors {
				if err.ID == diagnostics.UnreachableStatement {
					unreachable++
					continue
				}
				t.Fatalf("unexpected error: %v", err)
			}
			if unreachable != test.want {
				t.Fatalf("S3001 count = %d, want %d: %v", unreachable, test.want, errors)
			}
		})
	}
}

// A fact established before a loop does not prove an assertion inside the
// loop body that the body itself invalidates on later iterations, and the
// value a mutable binding has on the first analyzed iteration is not a
// constant.
//
// Rules:
//   - rules/errors/panic.md — § 15.6, § 15.8(1) proof on every path
func TestAssertionProofDoesNotCarryFactsAcrossLoopIterations(t *testing.T) {
	source := `module main

fn PreLoopFact(x: int, n: int) int {
    let mut v := x
    assert v <= 10
    let mut i := 0
    while i < n {
        assert v <= 10
        v += 5
        i += 1
    }
    return 2
}

fn FirstIterationValue(n: int) int {
    let mut i := 0
    while i < n {
        i += 1
        assert i == 1
    }
    return i
}

fn WhileConditionHolds(n: int) int {
    let limit := 10
    let mut i := 0
    while i < limit {
        assert i < limit
        i += 1
    }
    return i
}
`
	program := parser.New(lexer.New(source)).ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	graph := analyzer.CallGraph()
	// The loops also increment with `i += 1`, whose checked addition is an
	// independent arithmetic panic effect; only assertion effects are
	// compared here.
	for name, wantAssertionPanic := range map[string]bool{
		"PreLoopFact":         true,
		"FirstIterationValue": true,
		"WhileConditionHolds": false,
	} {
		got := false
		for _, effect := range graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).DirectEffects {
			if effect.Kind == EffectMayPanicAssertion {
				got = true
			}
		}
		if got != wantAssertionPanic {
			t.Errorf("%s assertion panic = %v, want %v", name, got, wantAssertionPanic)
		}
	}
}
