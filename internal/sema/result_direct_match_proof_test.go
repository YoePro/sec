package sema

import (
	"strings"
	"testing"
)

// An `Ok(...)` or `Err(...)` arm of a match directly on an immutable Result
// binding proves that state inside the arm, so the consuming projection of the
// same state may forget the unreachable alternate payload. The other arm, the
// code after the match, and a mutable Result keep the conservative rejection.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.1 "Consuming projections", §28
//   - rules/control-flow/flowcontrol_match.md — arm-local refinement
func TestResultProjectionUsesDirectMatchArmState(t *testing.T) {
	const consume = `
fn Consume(failure: LaunchFailure) void {}
`
	tests := []struct {
		name     string
		body     string
		rejected bool
	}{
		{name: "ok arm proves ok", body: `
fn Use() int {
    let result := Load()
    match result {
        Ok(_) => {
            discard result.Ok()
        }
        Err(failure) => {
            Consume(<-failure)
        }
    }
    return 0
}`},
		{name: "err arm does not prove ok", rejected: true, body: `
fn Use() int {
    let result := Load()
    match result {
        Ok(_) => {
            discard Work()
        }
        Err(failure) => {
            discard result.Ok()
            Consume(<-failure)
        }
    }
    return 0
}`},
		{name: "proof ends with the match", rejected: true, body: `
fn Use() int {
    let result := Load()
    match result {
        Ok(value) => {
            discard value
        }
        Err(failure) => {
            Consume(<-failure)
        }
    }
    discard result.Ok()
    return 0
}`},
		{name: "mutable result keeps rejection", rejected: true, body: `
fn Use() int {
    let mut result := Load()
    match result {
        Ok(_) => {
            discard result.Ok()
        }
        Err(failure) => {
            Consume(<-failure)
        }
    }
    return 0
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, resultStateProofPrelude+consume+test.body+"\n")
			rejected := false
			for _, err := range errors {
				if strings.Contains(err.Error(), "Ok() cannot discard the alternate LaunchFailure payload") {
					rejected = true
				} else {
					t.Fatalf("unexpected diagnostic: %v", err)
				}
			}
			if rejected != test.rejected {
				t.Fatalf("rejected=%v, want %v; errors: %v", rejected, test.rejected, errors)
			}
		})
	}
}
