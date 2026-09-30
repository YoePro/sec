package sema

import (
	"strings"
	"testing"
)

// TestDeferDependenciesPreserveDisjointStructPlaces verifies that a deferred
// field read blocks overlapping ownership invalidation without extending the
// dependency to a proven-disjoint sibling field.
//
// Rules:
//   - rules/control-flow/defer.md — §§8–11 and §29
//   - rules/memory/lifetime_analysis.md — §17.1–§17.2
//   - rules/memory/borrowing.md — §21(2), §21(5)
func TestDeferDependenciesPreserveDisjointStructPlaces(t *testing.T) {
	source := `module main

@noCopy
type Session struct { value: int }
type Pair struct { first: Session, second: Session }

fn Valid() void {
	let pair := Pair { first: Session { value: 1 }, second: Session { value: 2 } }
	defer {
		let observed := pair.first.value
		discard observed
	}
	let second :<- pair.second
	discard second
}

fn InvalidField() void {
	let pair := Pair { first: Session { value: 1 }, second: Session { value: 2 } }
	defer {
		let observed := pair.first.value
		discard observed
	}
	let first :<- pair.first
	discard first
}

fn InvalidWhole() void {
	let pair := Pair { first: Session { value: 1 }, second: Session { value: 2 } }
	defer {
		let observed := pair.first.value
		discard observed
	}
	let whole :<- pair
	discard whole
}
`
	errors := analyzeSourceRaw(t, source)
	if len(errors) != 2 {
		t.Fatalf("errors = %#v, want 2 overlapping defer dependency errors", errors)
	}
	if !strings.Contains(errors[0].Message, "cannot move pair.first while it is required by defer") {
		t.Fatalf("field diagnostic = %q", errors[0].Message)
	}
	if !strings.Contains(errors[1].Message, "cannot move pair while it is required by defer") {
		t.Fatalf("whole diagnostic = %q", errors[1].Message)
	}
}

// A dereference performed by defer depends on both the referenced Place and
// the reference value that carries its authority. Ordinary deferred value
// reads remain late reads and therefore still observe legal later assignment.
//
// Rules:
//   - rules/control-flow/defer.md — §§9–11
//   - rules/memory/borrowing.md — §21(1), §21(4)–(5)
func TestDeferRetainsReferenceHolderWithoutFreezingOrdinaryValues(t *testing.T) {
	source := `module main

type Item struct { value: int }

fn InvalidHolderMove() void {
	let item := Item { value: 1 }
	let view := ref item
	defer {
		let observed := view.value
		discard observed
	}
	let moved :<- view
	discard moved
}

fn InvalidHolderReplacement() void {
	let first := Item { value: 1 }
	let second := Item { value: 2 }
	let mut view := ref first
	defer {
		let observed := view.value
		discard observed
	}
	view = ref second
}

fn ValidLateValueRead() void {
	let mut value := 1
	defer {
		let observed := value
		discard observed
	}
	value = 2
}
`
	errors := analyzeSourceRaw(t, source)
	if len(errors) != 2 {
		t.Fatalf("errors = %#v, want move and replacement diagnostics", errors)
	}
	if !strings.Contains(errors[0].Message, "cannot move view while it is required by defer") {
		t.Fatalf("holder move diagnostic = %q", errors[0].Message)
	}
	if !strings.Contains(errors[1].Message, "cannot replace view while it is required as a reference holder by defer") {
		t.Fatalf("holder replacement diagnostic = %q", errors[1].Message)
	}
}
