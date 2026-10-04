package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A function value assigned in a nested branch may or may not have been
// reassigned when a later read executes, so the read observes the closed
// join of every assigned target: a panicking target reached only through one
// branch still violates @noPanic, two safe targets keep the proof, a write
// inside a loop makes the binding unknown, and straight-line rebinding stays
// exact.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable value flow", "Callable target sets", "Closed set", "Soundness of target sets"
//   - rules/errors/panic.md — § 21 "@noPanic"
func TestFunctionValueTargetsJoinAcrossControlFlow(t *testing.T) {
	const prelude = `module main

fn Safe(value: int) int {
	return value
}

fn AlsoSafe(value: int) int {
	let copy := value
	return copy
}

fn Risky(value: int) int {
	assert value > 3
	return value
}
`
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{name: "branch assigns a panicking target", wantError: "via Branch -> Risky", body: `
@noPanic
fn Branch(flag: bool, value: int) int {
	let mut operation: fn(int) int := Safe
	if flag {
		operation = Risky
	}
	return operation(value)
}`},
		{name: "else branch assigns a panicking target", wantError: "via Branch -> Risky", body: `
@noPanic
fn Branch(flag: bool, value: int) int {
	let mut operation: fn(int) int := Safe
	if flag {
		operation = AlsoSafe
	} else {
		operation = Risky
	}
	return operation(value)
}`},
		{name: "match arm assigns a panicking target", wantError: "via Branch -> Risky", body: `
@noPanic
fn Branch(maybe: Option[int], value: int) int {
	let mut operation: fn(int) int := Safe
	match maybe {
		Some(_) => {
			operation = Risky
		}
		None => {
			operation = AlsoSafe
		}
	}
	return operation(value)
}`},
		{name: "both branches safe", body: `
@noPanic
fn Branch(flag: bool, value: int) int {
	let mut operation: fn(int) int := Safe
	if flag {
		operation = AlsoSafe
	}
	return operation(value)
}`},
		{name: "loop write is unknown", wantError: "may-panic-unknown-callee", body: `
@noPanic
fn Looping(limit: int, value: int) int {
	let mut operation: fn(int) int := Safe
	let mut index := 0
	let mut total := 0
	while index < limit {
		total = operation(value)
		operation = AlsoSafe
		index += 1
	}
	return total
}`},
		{name: "straight-line rebinding stays exact", body: `
@noPanic
fn Straight(value: int) int {
	let mut operation: fn(int) int := Risky
	operation = Safe
	return operation(value)
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			if test.wantError == "" {
				if len(errors) != 0 {
					t.Fatalf("errors = %v, want none", errors)
				}
				return
			}
			if len(errors) != 1 || errors[0].ID != diagnostics.NoPanicViolation || !strings.Contains(errors[0].Message, test.wantError) {
				t.Fatalf("errors = %v, want one S1092 containing %q", errors, test.wantError)
			}
		})
	}
}
