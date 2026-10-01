package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Non-binding union state tests type as bool in if and while conditions and
// refine definite assignment: the false path of `is empty` and the true path
// of `is Variant` prove a possibly-empty binding initialized.
//
// Rules:
//   - rules/declarations/unions.md — §8.1 "Active variant test", §8.2 "Empty-state test", §8.3 "Data-flow refinement"
//   - rules/declarations/unions.md — §12 "Result and Option"
//   - rules/control-flow/flowcontrol_while.md — §8 "`is` state tests"
func TestUnionStateTestsTypeAndRefineAssignment(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/union_state_tests_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(input)); len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
}

// Without the refining test, the possibly-empty binding remains unassigned, so
// the refinement above is what makes the later use valid.
//
// Rules:
//   - rules/declarations/unions.md — §7.3 "Empty state is local initialization state", §8.3 "Data-flow refinement"
func TestUnionStateRefinementIsRequiredForUse(t *testing.T) {
	input := `
module main

type State union {
    Idle
    Running
}

fn Use(s: State) void {
}

fn Unrefined(flag: bool) void {
    let mut state: State
    if flag {
        state = State.Idle
    }
    if state is Running {
        return
    }
    Use(state)
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "variable state is unassigned") || errors[0].Line != 20 {
		t.Fatalf("errors = %+v, want the false path of is Running to leave state unassigned", errors)
	}
}

// Impossible empty tests use the canonical unreachable diagnostic, and
// unknown variants, mismatched qualifiers, non-union subjects, and moved-from
// bindings are rejected.
//
// Rules:
//   - rules/declarations/unions.md — §7.6 "Moved-from is not empty", §8.1 "Active variant test", §8.4 "Impossible state tests"
//   - rules/tooling/diagnostics.md — § 21 "Proven unreachable and dead code"
func TestUnionStateTestDiagnostics(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/union_state_tests_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	wants := []struct {
		line    int
		id      string
		message string
	}{
		{12, diagnostics.UnreachableStatement, "unreachable statement"},
		{17, "", "State has no variant Stopped"},
		{23, "", "is test qualifier Option does not match subject type State"},
		{29, "", "is Idle requires a union value, got int"},
		{37, "", "use of moved value source"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d", errors, len(wants))
	}
	for i, want := range wants {
		got := errors[i]
		if got.Line != want.line || !strings.Contains(got.Message, want.message) || want.id != "" && got.ID != want.id {
			t.Fatalf("error %d = %+v, want %q at line %d", i, got, want.message, want.line)
		}
	}
	if !strings.Contains(errors[0].Help, "always initialized") {
		t.Fatalf("impossible-test help = %q", errors[0].Help)
	}
}

// The compiler-known empty pattern is accepted over possibly-empty union
// bindings: variant arms prove the binding initialized, an unguarded catch-all
// covers the empty state, and termination and the published match plan include
// the empty arm.
//
// Rules:
//   - rules/declarations/unions.md — §10.1 "Empty pattern", §10.2 "Exhaustiveness with possible empty state"
//   - rules/declarations/unions.md — §10.3 "Known initialized subjects", §10.4 "Catch-all"
func TestUnionEmptyMatchPatternAccepted(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/union_empty_match_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(input))
	if len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
	emptyPlans := 0
	for _, plan := range analyzer.resolvedMatchPlans {
		if !plan.EmptyStateReachable {
			continue
		}
		emptyPlans++
		hasEmptyArm := false
		for _, arm := range plan.Arms {
			if arm.PatternKind == MatchPatternEmptyState {
				hasEmptyArm = true
			}
		}
		if !hasEmptyArm && len(plan.Arms) != 2 {
			t.Fatalf("empty-state plan without empty arm or catch-all: %+v", plan)
		}
	}
	if emptyPlans != 4 {
		t.Fatalf("empty-state plans = %d, want 4", emptyPlans)
	}
}

// Missing empty arms, empty arms on initialized subjects, duplicate empty arms,
// and value use inside the empty arm are rejected.
//
// Rules:
//   - rules/declarations/unions.md — §7.3 "Empty state is local initialization state"
//   - rules/declarations/unions.md — §10.2 "Exhaustiveness with possible empty state", §10.3 "Known initialized subjects"
func TestUnionEmptyMatchPatternDiagnostics(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/union_empty_match_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	wants := []struct {
		line    int
		message string
	}{
		{19, "match on state must handle its possibly empty state; add an empty arm or _"},
		{27, "unreachable empty match arm; state is always initialized here"},
		{42, "duplicate empty match arm"},
		{57, "variable state is unassigned"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d", errors, len(wants))
	}
	for i, want := range wants {
		if errors[i].Line != want.line || !strings.Contains(errors[i].Message, want.message) {
			t.Fatalf("error %d = %+v, want %q at line %d", i, errors[i], want.message, want.line)
		}
	}
}

// While conditions refine definite assignment from union state tests: the body
// of `while state is Variant` sees an initialized binding, and a loop over
// `is empty` without a reachable break leaves the binding initialized.
//
// Rules:
//   - rules/declarations/unions.md — §8.3 "Data-flow refinement"
//   - rules/control-flow/flowcontrol_while.md — §8 "`is` state tests", §23 "Definite assignment"
func TestWhileStateTestRefinesAssignment(t *testing.T) {
	input := `
module main

type State union {
    Idle
    Running
}

fn Use(s: State) void {
}

fn Next() bool {
    return true
}

fn InitializeInLoop(flag: bool) void {
    let mut state: State
    if flag {
        state = State.Idle
    }
    while state is empty {
        state = State.Running
    }
    Use(state)
}

fn VariantBody(flag: bool) void {
    let mut state: State
    if flag {
        state = State.Idle
    }
    while state is Idle {
        Use(state)
        state = State.Running
    }
}

fn BreakKeepsEmpty(flag: bool) void {
    let mut state: State
    if flag {
        state = State.Idle
    }
    while state is empty {
        if Next() {
            break
        }
        state = State.Running
    }
    Use(state)
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	if len(errors) != 1 || errors[0].Line != 49 || !strings.Contains(errors[0].Message, "variable state is unassigned") {
		t.Fatalf("errors = %+v, want only the break path to leave state unassigned", errors)
	}
	for expr, fact := range analyzer.resolvedStateTests {
		if !fact.MaybeEmpty || fact.StaticallyKnown {
			t.Fatalf("state test %s fact = %+v, want a possibly-empty observation that loop re-analysis does not overwrite", expr.String(), fact)
		}
	}
}
