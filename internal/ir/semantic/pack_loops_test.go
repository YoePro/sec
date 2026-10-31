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
	for _, want := range []string{"for-pack condition=", `"main::Size[T=int](ref T)"`, `"main::Size[T=bool](ref T)"`, "edge=continue"} {
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

// for ref mut over a ...ref mut pack keeps an explicit IR boundary until its
// per-position exclusive reborrow facts exist.
func TestPackLoopRefMutBoundary(t *testing.T) {
	_, err := buildPackage15Source(t, `module main
fn Touch[T](value: ref mut T) void {
}
fn All[...Types](values: ...ref mut Types) void {
    for ref mut value in values {
        Touch(value)
    }
}
fn main() int {
    let mut a := 1
    All(ref mut a)
    return 0
}
`, "")
	if err == nil || !strings.Contains(err.Error(), "for ref mut pack loop") {
		t.Fatalf("err = %v", err)
	}
}
