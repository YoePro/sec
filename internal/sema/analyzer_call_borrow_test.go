package sema

import (
	"strings"
	"testing"
)

// Reference arguments reserve their Places for the complete call after
// overload selection. Overlapping mutable/shared/direct uses are rejected,
// while shared/shared and proven-disjoint field reservations remain valid.
//
// Rules:
//   - rules/memory/borrowing.md — §15.2 call-site borrow creation
//   - rules/memory/borrowing.md — §15.4(1–5) argument order and overlap
func TestCallBoundedArgumentBorrowOverlap(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Pair struct { left: int, right: int }

fn MutateBoth(ref mut left: int, ref mut right: int) void {}
fn MutateAndShare(ref mut left: int, ref right: int) void {}
fn MutateAndRead(ref mut left: int, right: int) void {}
fn ShareBoth(ref left: int, ref right: int) void {}

fn InvalidMutableOverlap() void {
	let mut value := 1
	MutateBoth(value, value)
}

fn InvalidSharedOverlap() void {
	let mut value := 1
	MutateAndShare(value, value)
}

fn InvalidLaterRead() void {
	let mut value := 1
	MutateAndRead(value, value)
}

fn ValidDisjointAndShared() void {
	let mut pair := Pair { left: 1, right: 2 }
	MutateBoth(pair.left, pair.right)
	pair.left = 3
	ShareBoth(pair.left, pair.left)
}
`)
	if len(errors) != 3 {
		t.Fatalf("call-borrow diagnostics = %#v, want three overlap errors", errors)
	}
	if !strings.Contains(errors[0].Message, "argument 2 overlaps mutable borrow prepared for argument 1") {
		t.Fatalf("mutable/mutable diagnostic = %q", errors[0].Message)
	}
	if !strings.Contains(errors[1].Message, "argument 2 overlaps mutable borrow prepared for argument 1") {
		t.Fatalf("mutable/shared diagnostic = %q", errors[1].Message)
	}
	if !strings.Contains(errors[2].Message, "argument 2 reads value while mutable borrow prepared for argument 1 remains active") {
		t.Fatalf("mutable/read diagnostic = %q", errors[2].Message)
	}
}

// A selected reference parameter must also respect lexical borrows that were
// active before call evaluation; call reservations are additional constraints,
// not a replacement for ordinary borrow checking.
//
// Rules:
//   - rules/memory/borrowing.md — §15.2(4) selected parameter mode
//   - rules/memory/borrowing.md — §15.4 argument borrow commit
func TestCallBoundedArgumentBorrowChecksExistingBorrow(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Mutate(ref mut value: int) void {}

fn Check() void {
	let mut value := 1
	let shared := ref value
	Mutate(value)
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot create mutable reference to value while it is already borrowed") {
		t.Fatalf("existing-borrow diagnostics = %#v", errors)
	}
}
