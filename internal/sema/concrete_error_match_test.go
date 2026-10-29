package sema

import (
	"testing"

	"sec/internal/diagnostics"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 9.2–9.6, 9.11–9.13
//   - rules/control-flow/flowcontrol_match.md — "Concrete error variants"
//   - rules/errors/errorhandling.md — § 27.2b (MD-029: a bare payload-carrying
//     error variant is valid full-variant coverage with an Information)
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

fn EveryVariant() int {
    return match Get() {
        Ok(value) => value
        Err(Failure.Closed) => 1
        Err(Failure.Busy) => 2
        Err(Failure.Detail) => 3
    }
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
	advisories := 0
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OmittedPayloadPattern {
			advisories++
		}
	}
	if advisories != 2 {
		t.Fatalf("want two omitted-payload advisories for Err(Failure.Detail), got %+v", analyzer.Warnings())
	}
}
