package sema

import (
	"strings"
	"testing"
)

const resultStateProofPrelude = `
module main

fn Work() int {
    return 1
}

type LaunchFailure union error {
    Abandoned(Task[int]),
}

fn Load() Result[int, LaunchFailure] {
    return Ok(1)
}
`

// A consuming projection may forget a non-discardable alternate payload when
// a dominating borrowed-projection test, or construction by the matching
// Ok(...) or Err(...), proves that state unreachable on an immutable Result;
// otherwise the conservative rejection remains.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.1 "Consuming projections", §6.2, §28
//   - rules/control-flow/discard.md — "Recursive discardability"
func TestResultProjectionUsesPathSensitiveStateProof(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		rejected bool
	}{
		{name: "err ref is none", body: `
fn Use() int {
    let result := Load()
    if result.ErrRef is None {
        discard result.Ok()
    }
    return 0
}`},
		{name: "ok ref is not none", body: `
fn Use() int {
    let result := Load()
    if result.OkRef is not None {
        discard result.Ok()
    }
    return 0
}`},
		{name: "no proof", rejected: true, body: `
fn Use() int {
    let result := Load()
    discard result.Ok()
    return 0
}`},
		{name: "wrong test", rejected: true, body: `
fn Use() int {
    let result := Load()
    if result.OkRef is None {
        discard result.Ok()
    }
    return 0
}`},
		{name: "mutable binding", rejected: true, body: `
fn Use() int {
    let mut result := Load()
    if result.ErrRef is None {
        discard result.Ok()
    }
    return 0
}`}, {name: "constructed ok", body: `
fn Use() int {
    let result: Result[int, LaunchFailure] := Ok(5)
    discard result.Ok()
    return 0
}`},
		{name: "constructed err cannot project ok", rejected: true, body: `
fn Use() int {
    let worker := spawn Work()
    let result: Result[int, LaunchFailure] := Err(LaunchFailure.Abandoned(<-worker))
    discard result.Ok()
    return 0
}`},
		{name: "mutable constructed ok", rejected: true, body: `
fn Use() int {
    let mut result: Result[int, LaunchFailure] := Ok(5)
    discard result.Ok()
    return 0
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, resultStateProofPrelude+test.body+"\n")
			rejected := false
			for _, err := range errors {
				if strings.Contains(err.Message, "Ok() cannot discard the alternate LaunchFailure payload") {
					rejected = true
				}
			}
			if !test.rejected && len(errors) != 0 {
				t.Fatalf("errors = %v, want none", errors)
			}
			if rejected != test.rejected {
				t.Fatalf("errors = %v, rejected = %v, want %v", errors, rejected, test.rejected)
			}
		})
	}
}

// The Result state is also proven by the else branch of a borrowed-projection
// test, by the code after a branch that exits, by an `is Some(binding)` test,
// and by the `None`/`Some` arms of a match on a borrowed projection; the
// opposite state and mutable bindings stay rejected.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.1 "Consuming projections", §6.2, §28
//   - rules/control-flow/flowcontrol_match.md — arm-local refinement
func TestResultProjectionUsesBranchAndMatchStateProof(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		rejected bool
	}{
		{name: "else of ok ref is none", body: `
fn Use() int {
    let result := Load()
    if result.OkRef is None {
        return 1
    } else {
        discard result.Ok()
    }
    return 0
}`},
		{name: "after exiting err test", body: `
fn Use() int {
    let result := Load()
    if result.ErrRef is not None {
        return 1
    }
    discard result.Ok()
    return 0
}`},
		{name: "after non-exiting err test", rejected: true, body: `
fn Use() int {
    let result := Load()
    if result.ErrRef is not None {
        discard Work()
    }
    discard result.Ok()
    return 0
}`},
		{name: "after exiting else", body: `
fn Use() int {
    let result := Load()
    if result.ErrRef is None {
        discard Work()
    } else {
        return 1
    }
    discard result.Ok()
    return 0
}`},
		{name: "fact ends with the enclosing block", rejected: true, body: `
fn Use(flag: bool) int {
    let result := Load()
    if flag {
        if result.ErrRef is not None {
            return 1
        }
    }
    discard result.Ok()
    return 0
}`},
		{name: "some binding on ok ref", body: `
fn Use() int {
    let result := Load()
    if result.OkRef is Some(value) {
        discard result.Ok()
    }
    return 0
}`},
		{name: "match none arm on err ref", body: `
fn Use() int {
    let result := Load()
    match result.ErrRef {
        None => {
            discard result.Ok()
        }
        Some(_) => {
            discard Work()
        }
    }
    return 0
}`},
		{name: "match some arm proves the other state", rejected: true, body: `
fn Use() int {
    let result := Load()
    match result.ErrRef {
        None => {
            discard Work()
        }
        Some(_) => {
            discard result.Ok()
        }
    }
    return 0
}`},
		{name: "mutable binding in else", rejected: true, body: `
fn Use() int {
    let mut result := Load()
    if result.OkRef is None {
        return 1
    } else {
        discard result.Ok()
    }
    return 0
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, resultStateProofPrelude+test.body+"\n")
			rejected := false
			for _, err := range errors {
				if strings.Contains(err.Message, "Ok() cannot discard the alternate LaunchFailure payload") {
					rejected = true
				}
			}
			if !test.rejected && len(errors) != 0 {
				t.Fatalf("errors = %v, want none", errors)
			}
			if rejected != test.rejected {
				t.Fatalf("errors = %v, rejected = %v, want %v", errors, rejected, test.rejected)
			}
		})
	}
}
