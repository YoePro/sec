package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Field and element borrow rules of P15-61/P15-62: disjoint sibling fields
// and distinct constant elements may be mutably borrowed together, a read of a
// sibling path is not a read of the borrowed field, and a member name that
// equals a holder's name is not a use of that holder (borrow liveness).
//
// Rules:
//   - rules/memory/borrowing.md — §§ 4(1–3), 6, 8.1, 22
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 147, 148
func TestDisjointFieldAndElementBorrowsAreAccepted(t *testing.T) {
	input := `
module main

type Cell struct {
    v: int,
}

type Inner struct {
    count: Cell,
    limit: Cell,
}

fn Fields() int {
    let mut inner := Inner { count: Cell { v: 1 }, limit: Cell { v: 2 } }
    let count := ref mut inner.count
    let limit := ref mut inner.limit
    count.v = 5
    limit.v = 6
    return inner.limit.v
}

fn SiblingRead() int {
    let mut inner := Inner { count: Cell { v: 1 }, limit: Cell { v: 2 } }
    let count := ref mut inner.count
    let other := inner.limit.v
    count.v = other
    return 0
}

fn Elements() int {
    let mut grid := [Cell { v: 1 }, Cell { v: 2 }]
    let first := ref mut grid[0]
    let second := ref mut grid[1]
    first.v = 3
    second.v = 4
    return 0
}

fn Nested(outer: ref mut Inner) int {
    let count := ref mut outer.count
    count.v = 7
    return outer.count.v
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

// Overlapping field and element borrows, property borrows and constant
// zero-length element borrows are rejected, each with one identified
// diagnostic (P15-61, P15-62).
//
// Rules:
//   - rules/memory/borrowing.md — §§ 5.3, 6, 7, 22
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — § 86
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 43, 147, 148
func TestOverlappingFieldAndElementBorrowsAreRejected(t *testing.T) {
	for name, test := range map[string]struct {
		body string
		id   string
	}{
		"whole struct versus mutable field": {`
    let mut inner := Inner { count: Cell { v: 1 }, limit: Cell { v: 2 } }
    let count := ref mut inner.count
    let whole := ref inner
    count.v = 3`, diagnostics.BorrowSharedConflict},
		"property": {`
    let inner := Inner { count: Cell { v: 1 }, limit: Cell { v: 2 } }
    let total := ref inner.Total`, diagnostics.BorrowNonAddressable},
		"equal constant index": {`
    let mut grid := [Cell { v: 1 }, Cell { v: 2 }]
    let a := ref mut grid[1]
    let b := ref grid[1]
    a.v = 3`, diagnostics.BorrowSharedConflict},
		"dynamic indexes overlap": {`
    let mut grid := [Cell { v: 1 }, Cell { v: 2 }]
    let a := ref mut grid[i]
    let b := ref grid[j]
    a.v = 3`, diagnostics.BorrowSharedConflict},
		"zero-length constant index": {`
    let empty: Cell[0] := []
    let a := ref empty[0]`, diagnostics.IndexOutOfBounds},
	} {
		t.Run(name, func(t *testing.T) {
			input := `
module main

type Cell struct {
    v: int,
}

type Inner struct {
    count: Cell,
    limit: Cell,
}

impl Inner {
    property Total: int {
        get { return self.count.v + self.limit.v }
    }
}

fn Test(i: int, j: int) int {` + test.body + `
    return 0
}
`
			errors := analyzeSourceRaw(t, input)
			if len(errors) != 1 || errors[0].ID != test.id || errors[0].Help == "" {
				var messages []string
				for _, err := range errors {
					messages = append(messages, err.ID+" "+err.Message)
				}
				t.Fatalf("want one %s diagnostic with help, got %s", test.id, strings.Join(messages, "; "))
			}
		})
	}
}

// A reference returned by a call is a plain holder for borrow liveness: its
// origin comes from the callee's returned-origin summary, so the borrow of
// the argument ends at the holder's last use instead of the block end, and
// still conflicts while the holder is used (MD-061 § 4).
//
// Rules:
//   - rules/memory/borrowing.md — § 8.1 "Compiler-derived liveness"
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 61, 62
func TestCallReturnedReferenceHolderEndsAtLastUse(t *testing.T) {
	source := func(last string) string {
		return `
module main

type Cell struct {
    v: int,
}

type Pair struct {
    left: Cell,
    right: Cell,
}

fn Left(pair: ref Pair) ref Cell {
    return ref pair.left
}

fn RightMut(pair: ref mut Pair) ref mut Cell {
    return ref mut pair.right
}

fn Use() int {
    let mut pair := Pair { left: Cell { v: 1 }, right: Cell { v: 2 } }
    let left := Left(ref pair)
    let x := left.v
    let right := RightMut(ref mut pair)
    right.v = 5
    return ` + last + `
}
`
	}
	assertSemaErrors(t, analyzeSourceRaw(t, source("x")), nil)
	errors := analyzeSourceRaw(t, source("left.v"))
	if len(errors) != 1 || errors[0].ID != diagnostics.BorrowMutableConflict {
		t.Fatalf("want one %s conflict while left is still used, got %v", diagnostics.BorrowMutableConflict, errors)
	}
}
