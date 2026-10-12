package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// P15-48: a run-time validated use of a dynamic-epoch (Arena-backed)
// reference adds a may-panic-invalid-reference effect with the exact
// InvalidReferenceGeneration reason, so @noPanic rejects it; proven references,
// borrow creation and reference passing add no panic effect.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — P15-48
//   - rules/memory/reference_model.md — § 45
//   - rules/errors/panic.md — § 21 "@noPanic"
func TestDynamicReferenceUsesCarryInvalidReferencePanicEffect(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Pair struct { left: int, right: int }

@noPanic
fn Dynamic() Result[void, AllocationError] {
    let mut mem: Arena := Arena {}
    let values := try mem.Alloc[byte](4u)
    let element := ref values[0]
    discard element
    return Ok()
}

@noPanic
fn Proven(pair: ref Pair) int {
    return pair.left
}

@noPanic
fn Passing(pair: ref Pair) int {
    let copy := ref pair.left
    discard copy
    return Proven(pair)
}

@noPanic
fn Borrowing() int {
    let pair := Pair { left: 1, right: 2 }
    let first := ref pair
    return Proven(first)
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.NoPanicViolation || !strings.Contains(errors[0].Message, "function Dynamic") || !strings.Contains(errors[0].Help, "InvalidReferenceGeneration") {
		t.Fatalf("errors = %v, want only Dynamic rejected by @noPanic with the reference help", errors)
	}
	graph := analyzer.CallGraph()
	found := false
	for _, effect := range graph.EffectSummary(callGraphNodeIDByName(t, graph, "Dynamic")).DirectEffects {
		if effect.Kind == EffectMayPanicInvalidReference {
			found = true
			if len(effect.PanicReasonIDs) != 1 || effect.PanicReasonIDs[0] != diagnostics.PanicReasonInvalidReferenceGeneration {
				t.Fatalf("effect reasons = %v, want InvalidReferenceGeneration", effect.PanicReasonIDs)
			}
		}
	}
	if !found {
		t.Fatal("Dynamic has no may-panic-invalid-reference effect")
	}
	for _, name := range []string{"Proven", "Passing", "Borrowing"} {
		for _, effect := range graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).DirectEffects {
			if effect.Kind == EffectMayPanicInvalidReference {
				t.Fatalf("%s has an invalid-reference effect at %d:%d", name, effect.Source.Line, effect.Source.Column)
			}
		}
	}
	if !IsPanicEffectKind(EffectMayPanicInvalidReference) {
		t.Fatal("may-panic-invalid-reference is not classified as a panic effect")
	}
}
