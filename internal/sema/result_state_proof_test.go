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
// a dominating borrowed-projection test proves that state unreachable on an
// immutable Result; otherwise the conservative rejection remains.
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
