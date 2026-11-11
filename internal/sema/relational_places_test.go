package sema

import "testing"

// Relational reachability uses comparison facts per binding and on stored
// fields: a mutable binding changed only by direct assignment keeps its fact
// until its next write, and a by-value stored field keeps its fact until a
// field write or call barrier. Assignments, field writes, calls, computed
// getters, reference roots and mutable borrows never leave a stale fact.
//
// Rules:
//   - rules/tooling/diagnostics.md — § 21(1), (3)–(6) "Proven unreachable and dead code"
//   - rules/errors/panic.md — § 15.8(1) proof facts and invalidation
func TestRelationalReachabilityTracksBindingsAndStoredFields(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Box struct {
    count: int,
}

impl Box {
    property Doubled: int {
        get {
            return self.count * 2
        }
    }
}

fn Touch() void {}

fn LaterWrite(seed: int) int {
    let mut x := seed
    if x == 1 {
        if x == 2 {
            return 9
        }
    }
    x = 5
    return x
}

fn StoredField(b: Box) int {
    if b.count == 1 {
        if b.count == 2 {
            return 9
        }
    }
    return 0
}

fn AssignedBetween(seed: int) int {
    let mut x := seed
    if x == 1 {
        x = 2
        if x == 2 {
            return 9
        }
    }
    return x
}

fn FieldWrittenBetween(seed: Box) int {
    let mut b := seed
    if b.count == 1 {
        b.count = 2
        if b.count == 2 {
            return 9
        }
    }
    return 0
}

fn CallBetween(b: Box) int {
    if b.count == 1 {
        Touch()
        if b.count == 2 {
            return 9
        }
    }
    return 0
}

fn Getter(b: Box) int {
    if b.Doubled == 2 {
        if b.Doubled == 4 {
            return 9
        }
    }
    return 0
}

fn ReferenceRoot(b: ref Box) int {
    if b.count == 1 {
        if b.count == 2 {
            return 9
        }
    }
    return 0
}

fn Bump(v: ref mut int) void {}

fn Borrowed(seed: int) int {
    let mut x := seed
    if x == 1 {
        Bump(ref mut x)
        if x == 2 {
            return 9
        }
    }
    return x
}
`)
	assertSemaErrors(t, errors, []string{
		"unreachable statement at 22:13",
		"unreachable statement at 32:13",
	})
}
