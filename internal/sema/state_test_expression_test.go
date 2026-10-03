package sema

import (
	"strings"
	"testing"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 7.1–7.12, 7.16–7.18
//   - rules/foundations/operators.md — "State tests with is"
func TestStateTestsAreOrdinaryBoolExpressions(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type State union {
    Idle,
    Running,
}

fn Use() void {
}

fn Stored(state: State, option: Option[int], ready: bool) bool {
    let idle := state is Idle
    let missing := option is None
    let present := option is not None
    let busy := state is not Idle
    let some := option is not Some
    if ready && state is Idle {
        Use()
    }
    if option is Some(value) {
        Use()
    }
    while state is not Running && ready {
        return idle || missing || present || busy || some
    }
    return state is Running
}
`)
	if len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
}

func TestStateTestDiagnostics(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type State union {
    Idle,
    Running,
}

fn Bad(state: State, option: Option[int]) bool {
    let redundant := (state is Idle) == true
    let negated := !(state is Idle)
    let chained := state is Idle == true
    let bound := option is Some(value)
    let prefix := !state is Idle
    return redundant
}
`)
	wantErrors := []struct {
		line    int
		message string
	}{
		{11, "comparison chaining is not supported"},
		{12, "binds only as the complete if condition"},
		// § 7.16: ordinary prefix precedence makes this `(!state) is Idle`.
		{13, "operator ! requires bool operand"},
	}
	if len(errors) != len(wantErrors) {
		t.Fatalf("errors = %+v, want %d", errors, len(wantErrors))
	}
	for index, want := range wantErrors {
		if errors[index].Line != want.line || !strings.Contains(errors[index].Message, want.message) {
			t.Fatalf("error %d = %+v, want %q at line %d", index, errors[index], want.message, want.line)
		}
	}
	warnings := analyzer.Warnings()
	if len(warnings) != 2 || warnings[0].ID != "A2002" || warnings[0].Line != 9 || warnings[1].ID != "A2003" || warnings[1].Line != 10 ||
		!strings.Contains(warnings[1].Help, "state is not Idle") {
		t.Fatalf("warnings = %+v, want A2002 at 9 and A2003 at 10 recommending state is not Idle", warnings)
	}
}

func TestNegatedStateTestRefinesInitialization(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type State union {
    Idle,
    Running,
}

fn Use(state: State) void {
}

fn Refine(ready: bool) void {
    let mut state: State
    if ready {
        state = State.Idle
    }
    if state is not empty {
        Use(state)
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("errors = %+v, want is not empty to prove initialization on its true branch", errors)
	}
}
