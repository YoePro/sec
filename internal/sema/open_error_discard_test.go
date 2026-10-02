package sema

import (
	"strings"
	"testing"
)

const openErrorDiscardBody = `
fn Load() Result[int, error] {
    return Ok(1)
}

fn TryDiscard() int {
    return try Load() {
        Err(_) => 0
    }
}

fn MatchDiscard() int {
    match Load() {
        Ok(value) => return value
        Err(_) => return 0
    }
}

fn ExplicitDiscard(failure: error) void {
    discard failure
}

fn Projection() Option[int] {
    let result := Load()
    return result.Ok()
}
`

// The open error root is discardable only when no declared error type of the
// program carries a non-discardable obligation; every possible concrete
// payload must be discardable. Err(_) in try and match, explicit discard, and
// consuming projections share the rule.
//
// Rules:
//   - rules/control-flow/discard.md — "Recursive discardability"
//   - rules/errors/errorhandling.md — §17 "Err(_) and explicit error acknowledgement", §6.1
func TestOpenErrorDiscardabilityFollowsDeclaredErrors(t *testing.T) {
	if errors := analyzeSourceRaw(t, "\nmodule main\n"+openErrorDiscardBody); len(errors) != 0 {
		t.Fatalf("plain program errors = %v, want open error discardable", errors)
	}

	errors := analyzeSourceRaw(t, `
module main

fn Work() int {
    return 1
}

type LaunchFailure union error {
    Abandoned(Task[int]),
}
`+openErrorDiscardBody)
	found := 0
	for _, err := range errors {
		if strings.HasPrefix(err.Message, "function MatchDiscard must return") {
			continue // existing follow-on of a rejected match plan
		}
		if err.ID != "S1006" || !strings.Contains(err.Message, "error, which may carry LaunchFailure,") {
			t.Fatalf("diagnostic = %+v", err)
		}
		found++
	}
	if found != 4 {
		t.Fatalf("errors = %v, want four non-discardable open-error diagnostics", errors)
	}
}
