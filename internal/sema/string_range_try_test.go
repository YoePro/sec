package sema

import (
	"sec/internal/diagnostics"
	"testing"
)

// Every continuing recovery contributes its borrow origin, including handler
// locals; the try success origin alone cannot prove escape or mutation safety.
// Rules: rules/errors/errorhandling.md — §§9,21–22; borrowing.md — §5.5 (MD-126).
func TestStringRangeTryRecoveryOrigins(t *testing.T) {
	_, validErrors := stringRangeFixture(t, "try_valid")
	if len(validErrors) != 0 {
		t.Fatal(validErrors)
	}
	_, errors := stringRangeFixture(t, "try_origin_invalid")
	if len(errors) != 5 {
		t.Fatalf("expected four local escapes and a recovery-source mutation error: %v", errors)
	}
	for i, err := range errors {
		want := diagnostics.EscapeLocalStorage
		if i == 2 {
			want = diagnostics.BorrowMutationConflict
		}
		if err.ID != want || err.Help == "" {
			t.Fatalf("missing escape/borrow diagnostic: %+v", err)
		}
	}
}

// A range try changes only its own bounds failure into RangeError. It neither
// catches endpoint/handler panics nor suppresses static invalidity or a wrong
// propagation channel.
// Rules: rules/errors/errorhandling.md — §§9,11,31;
// rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md — §3.
func TestStringRangeTryFailureBoundaries(t *testing.T) {
	_, errors := stringRangeFixture(t, "try_invalid")
	if len(errors) != 4 {
		t.Fatalf("expected endpoint/handler panics, wrong channel and static invalidity: %v", errors)
	}
	want := map[string]int{diagnostics.NoPanicViolation: 2, diagnostics.TryPropagationIncompatible: 1, diagnostics.InvalidSliceRange: 1}
	for _, err := range errors {
		want[err.ID]--
		if err.Help == "" {
			t.Fatalf("missing explanation: %+v", err)
		}
	}
	for id, count := range want {
		if count != 0 {
			t.Fatalf("unexpected diagnostic distribution for %s: %v", id, errors)
		}
	}
}

// Resolved facts expose StringView/RangeError and all recovery origins without
// retaining handler-local symbol maps.
// Rules: rules/errors/errorhandling.md — §§9,21–22,35; rules/memory/borrowing.md — §5.5.
func TestStringRangeTryResolvedFacts(t *testing.T) {
	a, errors := stringRangeFixture(t, "try_valid")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	joined := 0
	for expr := range a.resolvedTries {
		fact, ok := a.ResolvedTryOf(expr)
		if !ok || fact.SuccessType.Kind != StringViewType || fact.ErrorType.Name != "RangeError" || len(fact.Failures) != 1 || fact.Failures[0].Kind != TryFailureSliceRange {
			t.Fatalf("incorrect range try fact: %+v", fact)
		}
		if origin, ok := a.expressionReferenceOrigins[expr]; ok {
			places := localOriginPlaces(origin)
			if len(places) == 2 {
				joined++
				if places[0].Root != "text" || places[1].Root != "fallback" {
					t.Fatalf("lost recovery backing: %+v", places)
				}
			}
		}
	}
	if joined != 3 {
		t.Fatalf("joined %d recovery origins, want 3", joined)
	}
	graph := a.CallGraph()
	for _, name := range []string{"TryFor", "Recover", "ReturnView"} {
		if graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).MayPanic {
			t.Fatalf("%s retains protected bounds panic", name)
		}
	}
}
