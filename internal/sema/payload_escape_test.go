package sema

import (
	"testing"

	"sec/internal/diagnostics"
)

// Every way a branch-scoped union, Option or Result payload reference could
// leave its match arm is rejected with an identified escape diagnostic and no
// follow-on errors: return (directly or inside an aggregate, for Ok and Err
// payloads), a store of a ref mut payload into an outer Place, a match
// expression producing the reference, and a returned lambda capturing it. A
// match whose arms all return still terminates the function when an arm
// reports such an error (P15-63).
//
// Rules:
//   - rules/memory/lifetime_analysis.md — §§ 1–4
//   - rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality"
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 44, 99
func TestBranchScopedPayloadReferenceEscapesAreRejected(t *testing.T) {
	input := `
module main

enum Problem error {
    Bad,
}

type Point struct { x: int }
type View struct { value: ref Point }
type MutView struct { value: ref mut Point }
type Pair union { Left(Point), Right(Point) }

fn ResultReturn(value: Result[Point, Problem], ref fallback: Point) View {
    match value {
        Ok(ref point) => { return View { value: point } }
        Err(_) => { return View { value: fallback } }
    }
}
fn ErrReturn(value: Result[Point, Problem], ref fallback: Problem) ref Problem {
    match value {
        Ok(_) => { return fallback }
        Err(ref problem) => { return problem }
    }
}
fn MutStore(target: ref mut MutView) void {
    let mut pair := Pair.Left(Point { x: 1 })
    match pair {
        Left(ref mut point) => { target.value = point }
        Right(_) => {}
    }
}
fn MatchResult(pair: Pair) int {
    let chosen := match pair {
        Left(ref point) => point
        Right(ref point) => point
    }
    return chosen.x
}
fn Capture(pair: Pair) fn() int {
    match pair {
        Left(ref point) => { return capture(point) fn() int { return point.x } }
        Right(_) => { return fn() int { return 0 } }
    }
}
`
	errors := analyzeSourceRaw(t, input)
	got := map[string]int{}
	for _, err := range errors {
		got[err.ID]++
		if err.ID == "" || err.Help == "" {
			t.Errorf("escape diagnostic without identity or help: %+v", err)
		}
	}
	if got[diagnostics.EscapeMatchPayload] != 4 || got[diagnostics.EscapeClosureCapture] != 1 || len(errors) != 5 {
		t.Fatalf("want four %s and one %s diagnostics, got %v", diagnostics.EscapeMatchPayload, diagnostics.EscapeClosureCapture, errors)
	}
}
