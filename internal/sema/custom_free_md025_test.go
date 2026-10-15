package sema

import (
	"strings"
	"testing"
)

const md025Prelude = `
module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
        if self.id == 0 {
            return
        }
        Release(self.id)
    }
}

fn Release(id: int) void {
}

fn OpenHandle() Handle {
    return Handle { id: 1 }
}

type Holder struct {
    inner: Handle,
}

type Pair struct {
    first: Holder,
    count: int,
}

impl Pair {
    free {
        Release(self.count)
    }
}
`

func md025Errors(t *testing.T, body string) []string {
	t.Helper()
	var messages []string
	for _, err := range analyzeSourceRaw(t, md025Prelude+body) {
		messages = append(messages, err.Message)
	}
	return messages
}

func expectMessages(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("want %d errors %v, got %d:\n%s", len(want), want, len(got), strings.Join(got, "\n"))
	}
	for _, fragment := range want {
		found := false
		for _, message := range got {
			found = found || strings.Contains(message, fragment)
		}
		if !found {
			t.Errorf("missing %q in:\n%s", fragment, strings.Join(got, "\n"))
		}
	}
}

// MD-025.1: a type with custom free is MoveOnly even when its fields are plain
// integers; owning aggregates inherit it, an explicit move is accepted, and
// free does not imply must-use (an unused value is destroyed normally). The
// classification does not depend on source order, also through impl extends.
//
// Rules:
//   - rules/memory/copy_move.md — § 3.5 "Custom free"
func TestCustomFreeTypesAreMoveOnly(t *testing.T) {
	got := md025Errors(t, `
fn Copies() void {
    let first := OpenHandle()
    let second := first
    let moved :<- second
    let holder := Holder { inner: OpenHandle() }
    let holderCopy := holder
    let handles := [OpenHandle(), OpenHandle()]
    let handlesCopy := handles
    let maybe: Option[Handle] := Some(OpenHandle())
    let maybeCopy := maybe
    discard OpenHandle()
    let unused := OpenHandle()
}
`)
	expectMessages(t, got,
		"cannot copy move-only value first: type Handle defines free, so it is MoveOnly",
		"cannot copy move-only value holder: it owns Handle, which defines free",
		"cannot copy move-only value handles: it owns Handle, which defines free",
		"cannot copy move-only value maybe: it owns Handle, which defines free",
	)

	extended := analyzeSourceRaw(t, `
module main

fn Copy(value: Late) void {
    let other := value
}

type Late struct {
    id: int,
}

impl Late {
}

impl extends Late {
    free {
    }
}
`)
	if len(extended) != 1 || !strings.Contains(extended[0].Message, "type Late defines free, so it is MoveOnly") {
		t.Fatalf("free in a later impl extends must make Late MoveOnly: %+v", extended)
	}
}

// MD-025.2 and MD-025.5: an owned field or nested owned sub-Place of a
// custom-free value cannot be discarded on its own, as it cannot be moved;
// inside free, discarding a nontrivial field of self is a provable double
// release because automatic field destruction follows free. Whole-value
// discard stays legal, and discarding it again is a no-op.
func TestCustomFreeFieldDiscardFollowsPartialMoveRules(t *testing.T) {
	got := md025Errors(t, `
fn Fields() void {
    let pair := Pair { first: Holder { inner: OpenHandle() }, count: 1 }
    discard pair.first
    discard pair.first.inner
    discard pair.count
    let moved :<- pair.first
    let read := pair.count
}

fn Whole() void {
    let pair := Pair { first: Holder { inner: OpenHandle() }, count: 1 }
    discard pair
    discard pair
}

type Owner struct {
    holder: Holder,
}

impl Owner {
    free {
        discard self.holder
    }
}
`)
	expectMessages(t, got,
		"cannot discard owned field pair.first of pair: type Pair defines free",
		"cannot discard owned field pair.first.inner of pair: type Pair defines free",
		"cannot discard owned field pair.count of pair: type Pair defines free",
		"cannot move pair.first out of pair because type Pair defines custom free",
		"cannot discard self.holder inside free: automatic field destruction after free would release it again",
	)
}

// MD-025.4 and MD-025.6: free is rejected on a unit, accepted on a register
// target, allows bare return on any path and rejects a returned value. The
// existing restrictions (defer inside free, direct free calls) still hold.
func TestCustomFreeTargetsAndEarlyReturn(t *testing.T) {
	got := md025Errors(t, `
unit rpm physical

impl <rpm> {
    Dimension: [time^-1]
    Scale: 1
    free {
    }
}

type Status register[8] {
    Ready: bit[8],
}

impl Status {
    free {
    }
}

type Early struct {
    id: int,
}

impl Early {
    free {
        if self.id > 1 {
            if self.id > 2 {
                return
            }
            return
        }
    }
}

type Valued struct {
    id: int,
}

impl Valued {
    free {
        return 1
    }
}

type Deferring struct {
    id: int,
}

impl Deferring {
    free {
        defer {
            Release(self.id)
        }
    }
}
`)
	expectMessages(t, got,
		"free is not permitted on a unit",
		"'return' in 'free' cannot return a value",
		"defer",
	)
}
