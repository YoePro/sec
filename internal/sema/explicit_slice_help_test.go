package sema

import (
	"strings"
	"testing"
)

// collections.md § 7.3: passing T[N], T[] or a reference to one where a
// slice is expected stays an error, now with help naming the explicit
// borrow `ref values[..]` (mutable slices `ref mut values[..]`); a
// different element type gets no slice suggestion.
func TestExplicitSliceHelpOnArgumentMismatch(t *testing.T) {
	errs := analyzeSourceRaw(t, `module main

fn Count(items: ref int[]) uint {
    return len(items)
}

fn Fill(items: ref mut int[]) void {
    items[0] = 1
}

fn main() int {
    let mut fixed: int[2] := [1, 2]
    fixed[1] = 4
    let dynamic: int[] := []
    let other: bool[2] := [true, false]
    discard Count(fixed)
    discard Count(ref fixed)
    discard Count(dynamic)
    Fill(ref mut fixed)
    discard Count(other)
    discard Count(ref fixed[..])
    return 0
}
`)
	if len(errs) != 5 {
		t.Fatalf("errors = %v", errs)
	}
	for index, want := range []string{"`ref fixed[..]`", "`ref fixed[..]`", "`ref dynamic[..]`", "`ref mut fixed[..]`"} {
		if !strings.Contains(errs[index].Help, want) || !strings.Contains(errs[index].Help, "no implicit array-to-slice conversion") {
			t.Errorf("error %d help = %q, want %s", index, errs[index].Help, want)
		}
	}
	if strings.Contains(errs[4].Help, "[..]") {
		t.Errorf("element mismatch suggested a slice: %q", errs[4].Help)
	}
}
