package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Hover over a transfer site shows the copy/move operation Sema resolved, the
// copy classification of the transferred type, the owning destination (with
// the consuming parameter of a call) and the source state after the transfer.
// Rules: rules/memory/copy_move.md — §§ 3, 8–11, 29(1)–(2); rules/tooling/lsp.md — "Hover".
func TestOwnershipOperationHover(t *testing.T) {
	text := `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

type Pair struct {
    a: Res,
    b: Res,
}

fn Consume(-> r: Res) int {
    return r.id
}

fn Keep(value: int) int {
    return value
}

type Point struct {
    x: int,
}

fn Look(point: Point) int {
    return point.x
}

fn Main() int {
    let r := Res { id: 1 }
    let n := 3
    let k := Keep(n)
    let copied := n
    let pt := Point { x: 1 }
    let pt2 := pt
    let seen := Look(pt)
    let moved :<- r
    let x := Consume(<-moved)
    let p := Pair { a: Res { id: 2 }, b: Res { id: 3 } }
    let first :<- p.a
    let take := capture(<-first) fn() Res {
        return first
    }
    return k + x + copied
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	cases := []struct {
		marker string
		want   []string
	}{
		{"Keep(n", []string{"trivial copy of `n`", "copy classification: trivial (`int`)", "parameter `value` of `Keep` (argument 1)", "source after: still owned"}},
		{"pt2 := pt", []string{"trivial copy of `pt`", "trivial (`Point`)", "initialization `pt2`"}},
		{"Look(pt", []string{"parameter `point` of `Look`", "still owned"}},
		{"<- r", []string{"operation: move of `r`", "move-only (`Res`)", "initialization `moved`", "source after: moved"}},
		{"Consume(<-moved", []string{"move of `moved`", "consuming parameter `r` of `Consume` (argument 1)", "source after: moved"}},
		{"p.a", []string{"partial move of `p.a`", "initialization `first`"}},
		{"(<-first", []string{"move of `first`", "closure environment capture"}},
	}
	for _, test := range cases {
		idx := strings.Index(text, test.marker)
		if idx < 0 {
			t.Fatalf("marker %q missing", test.marker)
		}
		hover, ok := hoverForSource(uri, text, offsetPosition(text, idx+len(test.marker)-1))
		if !ok || !strings.Contains(hover.Contents.Value, "**Ownership**") {
			t.Fatalf("%q: no ownership hover: %q", test.marker, hover.Contents.Value)
		}
		for _, want := range test.want {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("%q: hover lacks %q:\n%s", test.marker, want, hover.Contents.Value)
			}
		}
	}
	// A plain observation is not a transfer and gets no ownership section.
	idx := strings.Index(text, "return k + x")
	if hover, _ := hoverForSource(uri, text, offsetPosition(text, idx+len("return k"))); strings.Contains(hover.Contents.Value, "**Ownership**") {
		t.Errorf("observation got ownership hover: %s", hover.Contents.Value)
	}
}
