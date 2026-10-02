package sema

import (
	"strings"
	"testing"
)

const availabilityDiagnosticPrelude = `
module main

@noCopy
type Resource struct {
    raw: int,
}

type Pair struct {
    first: Resource,
    second: Resource,
}

fn Consume(-> value: Resource) void {
    discard value
}

fn Read(value: ref Resource) int {
    return value.raw
}
`

// Unavailable-use diagnostics distinguish the availability state from the
// retained UnavailableReason provenance: a Place unavailable on only some
// continuing paths is reported as possibly unavailable with availability-test
// help, a definite move keeps use-after-move guidance, every possible reason
// survives a join, and partial availability has its own stable identity.
//
// Rules:
//   - rules/memory/ownership.md — §5 "Conditional availability", §6 "Unavailability reason"
//   - rules/memory/ownership.md — §33.2 "Use after move", §33.3 "Conditional availability"
//   - rules/memory/destruction.md — §11.1 "Conditional state"
//   - rules/tooling/diagnostics.md — stable registered IDs
func TestUnavailablePlaceDiagnosticsSeparateAvailabilityFromReason(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		id       string
		message  string
		help     string
		previous int
	}{
		{
			name: "conditional read",
			body: `
fn Use(ready: bool) void {
    let value := Resource { raw: 1 }
    if ready {
        Consume(<-value)
    }
    Consume(<-value)
}`,
			id: "S1089", message: "value may no longer be available here; it was consumed by a call on one possible execution path",
			help: "test `value is available` before using it", previous: 25,
		},
		{
			name: "conditional nested branch",
			body: `
fn Use(first: bool, second: bool) void {
    let value := Resource { raw: 1 }
    if first {
        if second {
            Consume(<-value)
        }
    } else {
        Consume(<-value)
    }
    discard Read(ref value)
}`,
			id: "S1089", message: "cannot borrow conditionally available place value; it was consumed by a call on one possible execution path",
		},
		{
			name: "every path with distinct reasons",
			body: `
fn Use(ready: bool) void {
    let value := Resource { raw: 1 }
    if ready {
        Consume(<-value)
    } else {
        discard value
    }
    Consume(<-value)
}`,
			id: "S1088", message: "value value is no longer available; on every possible execution path it was consumed by a call or discarded",
			help: "reinitialize a mutable binding",
		},
		{
			name: "definite move",
			body: `
fn Use() void {
    let value := Resource { raw: 1 }
    let moved :<- value
    discard moved
    Consume(<-value)
}`,
			id: "S1088", message: "use of moved value value", help: "use value before that operation",
		},
		{
			name: "definite partial",
			body: `
fn Use(pair: Pair) void {
    let first :<- pair.first
    discard first
    discard pair
}`,
			id: "S1090", message: "cannot use partially moved value pair; place pair.first is unavailable",
			help: "use the still-available sub-places individually",
		},
		{
			name: "conditional partial",
			body: `
fn Use(pair: Pair, ready: bool) void {
    if ready {
        Consume(<-pair.first)
    }
    discard pair
}`,
			id: "S1089", message: "pair may no longer be available here; sub-place pair.first was moved on one possible execution path",
			help: "test `pair.first is available`",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, availabilityDiagnosticPrelude+test.body+"\n")
			if len(errors) != 1 {
				t.Fatalf("errors = %v, want one %s", errors, test.id)
			}
			err := errors[0]
			if err.ID != test.id || err.Message != test.message {
				t.Fatalf("diagnostic = %s %q, want %s %q", err.ID, err.Message, test.id, test.message)
			}
			if test.help != "" && !strings.Contains(err.Help, test.help) {
				t.Fatalf("help = %q, want it to contain %q", err.Help, test.help)
			}
			if err.PreviousLine == 0 || (test.previous != 0 && err.PreviousLine != test.previous) {
				t.Fatalf("related location = %d, want the earlier ownership operation (%d)", err.PreviousLine, test.previous)
			}
		})
	}
}

// Reason provenance never blocks convergence: availability refinement,
// discard of a possibly or definitely unavailable Place, and reinitialization
// after a mixed-reason join remain valid.
//
// Rules:
//   - rules/memory/ownership.md — §§20–23 refinement and discard convergence, §24 reinitialization
//   - rules/memory/destruction.md — §12.2, §12.3, §13.1, §13.3
func TestAvailabilityReasonsDoNotBlockConvergence(t *testing.T) {
	errors := analyzeSourceRaw(t, availabilityDiagnosticPrelude+`
fn Refine(ready: bool) void {
    let value := Resource { raw: 1 }
    if ready {
        Consume(<-value)
    }
    if value is available {
        Consume(<-value)
    }
}

fn Converge(ready: bool) void {
    let value := Resource { raw: 1 }
    if ready {
        Consume(<-value)
    } else {
        discard value
    }
    discard value
}

fn Reinitialize(ready: bool) void {
    let mut value := Resource { raw: 1 }
    if ready {
        Consume(<-value)
    } else {
        discard value
    }
    value = Resource { raw: 2 }
    Consume(<-value)
}

fn Repair(ready: bool) void {
    let mut value := Resource { raw: 1 }
    if ready {
        Consume(<-value)
    }
    value = Resource { raw: 2 }
    Consume(<-value)
}
`)
	assertSemaErrors(t, errors, nil)
}
