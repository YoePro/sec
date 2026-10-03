package sema

import (
	"strings"
	"testing"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 9.2–9.6, 9.11–9.13
//   - rules/control-flow/flowcontrol_match.md — "Concrete error variants"
func TestMatchConcreteUnionErrorVariants(t *testing.T) {
	source := `module main

type Failure union error {
    Closed,
    Busy,
    Detail(string),
}

fn Get() Result[int, Failure] {
    return Ok(1)
}

fn Covered() int {
    return match Get() {
        Ok(value) => value
        Err(Failure.Closed) => 1
        Err(Failure.Busy) => 2
        Err(failure) => 3
    }
}

fn PayloadVariant() int {
    return match Get() {
        Ok(value) => value
        Err(Failure.Detail) => 1
        Err(_) => 2
    }
}
`
	errors := analyzeSourceRaw(t, source)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot select a payload-carrying error variant") {
		t.Fatalf("errors = %+v, want one payload-carrying variant diagnostic", errors)
	}
}
