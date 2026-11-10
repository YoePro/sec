package semantic

import (
	"strings"
	"testing"
)

// The unrolled pack loop keeps one loop record with a position cursor, a
// per-position body built with that position's concrete element, ordinary
// continue/break edges, and an empty pack that never enters the body.
func TestPackLoopBuildsPerPositionBodies(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
fn Size[T](value: ref T) uint {
    return 1
}
fn Count[...Types](values: ...ref Types) uint {
    let mut n: uint := 0
    for index, value in values {
        if index == 0 {
            continue
        }
        n = n + Size(value)
    }
    return n
}
fn Empty[...Types](values: ...ref Types) uint {
    let mut n: uint := 0
    for ref value in values {
        n = n + 1
    }
    return n
}
fn main() int {
    let a := 1
    let b := true
    let c := Count(ref a, ref b)
    let d := Empty()
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"for-pack condition=", `"main::Size[T=int](ref int)"`, `"main::Size[T=bool](ref bool)"`, "edge=continue"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	empty := ""
	for _, function := range module.Functions {
		if strings.HasPrefix(string(function.ID), "main::Empty[") {
			empty = Format(&Module{Types: module.Types, Functions: []*Function{function}})
		}
	}
	if !strings.Contains(empty, "for-pack condition=") || !strings.Contains(empty, "body=none") {
		t.Fatalf("empty pack loop must not enter its body:\n%s", empty)
	}
}

// MD-117 binding modes per position: for ref mut reborrows each `ref mut`
// parameter, and for ref over an owned pack borrows the callee-owned value
// through a read-only Place root.
// Rules: rules/declarations/functions.md §38A(5)(4)-(6); rules/compiler/semantic_ir.md §27A(5).
func TestPackLoopBindingModes(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
fn Touch[T](value: ref T) void {
}
fn TouchMut[T](value: ref mut T) void {
}
fn All[...Types](values: ...ref mut Types) void {
    for ref mut value in values {
        TouchMut(value)
        TouchMut(value)
    }
}
fn Owned[...Types](values: ...Types) void {
    for ref value in values {
        Touch(value)
    }
}
fn main() int {
    let mut a := 1
    let mut b := true
    All(ref mut a, ref mut b)
    a = a + 1
    Owned(1, true)
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"ref.reborrow-mut %0", "ref.reborrow-mut %1", `"main::TouchMut[T=int](ref mut int)"`, `"main::TouchMut[T=bool](ref mut bool)"`, "place.value-root %0", "place.value-root %1", "ref.borrow-shared", `"main::Touch[T=bool](ref bool)"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

// Constant pack indexing selects the expanded parameter with its concrete type,
// through a holder pass, a reborrow, a value copy or a value-root borrow.
func TestPackIndexSelectsPosition(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
fn Touch[T](value: ref T) void {
}
fn TouchMut[T](value: ref mut T) void {
}
fn Keep[T](value: T) void {
}
fn Shared[...Types](values: ...ref Types) void {
    Touch(values[1])
    let held := values[0]
    let again := ref values[0]
    Touch(held)
    Touch(again)
}
fn Exclusive[...Types](values: ...ref mut Types) void {
    TouchMut(values[0])
    let r := ref mut values[0]
    TouchMut(r)
}
fn Owned[...Types](values: ...Types) uint {
    Touch(ref values[1])
    let first := values[0]
    Keep(values[1])
    return values.Len
}
fn main() int {
    let a := 1
    let mut b := true
    Shared(ref a, ref b)
    Exclusive(ref mut b)
    let n := Owned(1, true)
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{`"main::Touch[T=bool](ref bool)"`, `"main::Touch[T=int](ref int)"`, `"main::TouchMut[T=bool](ref mut bool)"`, "ref.reborrow-shared %0", "ref.reborrow-mut %0", "place.value-root %1", `"main::Keep[T=bool](bool)"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

// Nested pack loops bind each loop's own element, so inner generic calls see
// the inner position's type; switch, try and panic keep their ordinary edges.
func TestPackLoopBodyControlFlow(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
fn Size[T](value: ref T) uint {
    return 1
}
fn Fail() Result[int, StringError] {
    return Ok(1)
}
fn Pairs[...Types](values: ...ref Types) uint {
    let mut n: uint := 0
    for ref a in values {
        for ref b in values {
            n = n + Size(a) + Size(b)
        }
    }
    return n
}
fn Body[...Types](values: ...ref Types) Result[int, StringError] {
    let mut n := 0
    for ref value in values {
        n = try Fail()
        switch n {
            case 2: continue
            case 3: break
            default: n = n + 1
        }
        if n == 9 {
            panic "nine"
        }
        if n == 8 {
            return Ok(n)
        }
    }
    return Ok(n)
}
fn main() int {
    let a := 1
    let b := true
    let c := Pairs(ref a, ref b)
    let d := Body(ref a, ref b)
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{`"main::Size[T=int](ref int)"`, `"main::Size[T=bool](ref bool)"`, "panic reason=ExplicitPanic"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	if count := strings.Count(text, "for-pack condition="); count < 4 {
		t.Fatalf("for-pack records = %d:\n%s", count, text)
	}
}
