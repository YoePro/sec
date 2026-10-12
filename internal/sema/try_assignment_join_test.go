package sema

import (
	"strings"
	"testing"
)

// A handled try assignment joins its resolved edges instead of analyzing the
// handlers in sequence after the store: the success edge reinitializes the
// target, every handler starts from the failure edge where the target keeps
// its previous state, and only handlers that complete normally reach the
// statement join. Effects of a returning handler never leak into another
// handler or into the continuation.
//
// Rules:
//   - rules/errors/errorhandling.md — §15 "Local try handlers", §23 "Fallible assignment"
//   - rules/memory/ownership.md — §§20–21 control-flow merge
func TestTryAssignmentJoinsResolvedEdges(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Label string minLen 1

enum SetterError error { Rejected, Busy }

type Vehicle struct { speed: int }

impl Vehicle {
	property Speed: int {
		get { return speed }
		try set value SetterError { return Err(SetterError.Rejected) }
	}
}

fn Release(-> label: Label) void {}
fn Drop(-> text: string) void {}

fn MaybeReinitialized(raw: string, label: Label) void {
    let mut current := label
    Release(<-current)
    try current = Label(raw) {
        Err(error) => { discard error }
    }
    Release(<-current)
}

fn Reinitialized(raw: string, label: Label) Result[void, ContractError] {
    let mut current := label
    Release(<-current)
    try current = Label(raw) {
        Err(error) => { return Err(error) }
    }
    Release(<-current)
    return Ok()
}

fn ReturningHandlerMove(raw: string, label: Label, text: string) Result[void, ContractError] {
    let mut current := label
    try current = Label(raw) {
        Err(error) => {
            Drop(<-text)
            return Err(error)
        }
    }
    Drop(<-text)
    Release(<-current)
    return Ok()
}

fn SeparateHandlers(vehicle: Vehicle, text: string) Result[void, SetterError] {
    try vehicle.Speed = 10 {
        Err(SetterError.Rejected) => {
            Drop(<-text)
            return Err(SetterError.Rejected)
        }
        Err(SetterError.Busy) => {
            Drop(<-text)
        }
    }
    return Ok()
}
`)
	if len(errors) != 1 || errors[0].Line != 26 || !strings.Contains(errors[0].Message, "current may no longer be available") {
		t.Fatalf("errors = %v, want one maybe-consumed diagnostic for the continuing handler edge", errors)
	}
}
