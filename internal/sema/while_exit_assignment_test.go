package sema

import (
	"strings"
	"testing"
)

// flowcontrol_while.md § 28: a binding is definitely assigned after a while
// loop when every reachable exit assigns it — the zero-iteration exit (absent
// when the first condition test is proven true), the condition-false exit
// after an iteration (body end and continue edges), and every reachable break.
func TestWhileExitDefiniteAssignment(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type State union {
    Idle
    Running
}

fn Use(value: State) void {}

fn FirstIterationProven() void {
    let mut value: State
    let mut remaining := 3
    while remaining > 0 {
        value = State.Idle
        remaining -= 1
    }
    Use(value)
}

fn ContinueEdgeAssigns(flag: bool) void {
    let mut value: State
    let mut remaining := 2
    while remaining > 0 {
        remaining -= 1
        if flag {
            value = State.Idle
            continue
        }
        value = State.Running
    }
    Use(value)
}

fn BreakAndConditionExits(flag: bool) void {
    let mut value: State
    let mut remaining := 2
    while remaining > 0 {
        value = State.Idle
        if flag {
            break
        }
        remaining -= 1
    }
    Use(value)
}

fn MayRunZeroTimes(count: int) void {
    let mut value: State
    let mut remaining := count
    while remaining > 0 {
        value = State.Idle
        remaining -= 1
    }
    Use(value)
}

fn BreakBeforeAssignment(flag: bool) void {
    let mut value: State
    let mut remaining := 2
    while remaining > 0 {
        if flag {
            break
        }
        value = State.Idle
        remaining -= 1
    }
    Use(value)
}

fn ContinueBeforeAssignment(flag: bool) void {
    let mut value: State
    let mut remaining := 2
    while remaining > 0 {
        remaining -= 1
        if flag {
            continue
        }
        value = State.Idle
    }
    Use(value)
}
`)
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	if len(errors) != 3 {
		t.Fatalf("want exactly the three unassigned uses, got:\n%s", joined)
	}
	for _, line := range []string{"55:9", "68:9", "81:9"} {
		if !strings.Contains(joined, line) {
			t.Fatalf("missing unassigned use at line %s in:\n%s", line, joined)
		}
	}
}
