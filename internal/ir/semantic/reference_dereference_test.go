package semantic

import (
	"strings"
	"testing"
)

// MD-063: `*r` resolves to the referent Place before Semantic IR generation;
// a read and a whole-value replacement are separate Place operations.
func TestExplicitDereferenceLowersThroughReferentPlace(t *testing.T) {
	source := `
module main

fn Increment(value: ref mut int) void {
    *value += 1
}

fn Read(value: ref int) int {
    return *value * 2
}

fn main() int {
    let mut number := 10
    let r := ref mut number
    *r = 15
    *r += 1
    let copy := *r
    Increment(number)
    return copy + Read(number) - 50
}
`
	module, err := buildPackage15Source(t, source, "")
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	increment := text[strings.Index(text, "Increment("):strings.Index(text, "Read(")]
	// `*value += 1` forms the referent Place once, reads it, and writes the
	// checked result to that same Place only on the non-failing edge.
	if strings.Count(increment, "ref.deref-place") != 1 || !strings.Contains(increment, "%2 = place.read %1") || !strings.Contains(increment, "place.write %1, %4") {
		t.Fatalf("compound write through *value did not reuse one referent Place:\n%s", increment)
	}
	if strings.Index(increment, "place.write") < strings.Index(increment, "fail.arithmetic") {
		t.Fatalf("compound write committed before the arithmetic check:\n%s", increment)
	}
	for _, want := range []string{"place.write %3, %4", "%13 = place.read %12", "%2 = place.read %1 : !2"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}
