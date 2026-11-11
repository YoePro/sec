package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Hover over a reference creation, or over the binding that holds it, shows
// the borrow kind, referent, holder, reborrow parent, authority, the resolved
// end of the holder's live range and the reference's validity dependencies.
// Rules: rules/memory/borrowing.md — §§ 11, 15; rules/tooling/lsp.md — "Hover".
func TestBorrowHover(t *testing.T) {
	text := `module main

type Point struct {
    x: int,
    y: int,
}

fn Read(p: ref Point) int {
    return p.x
}

fn Bump(p: ref mut Point) {
    p.x = p.x + 1
}

fn Main() int {
    let mut point := Point { x: 1, y: 2 }
    let view := ref point
    let seen := view.x
    let again := ref view.y
    let total := seen + *again
    Bump(ref mut point)
    let n := Read(point)
    return total + n
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	cases := []struct {
		marker string
		skip   int
		want   []string
	}{
		{"ref point", 0, []string{"**Borrow**", "kind: shared borrow (`ref`)", "referent: `point`", "holder: `view` (declared at 18:9)", "authority: read-only", "storage domain: automatic storage of `point` in this invocation", "ends: after its last required use, before 22:5", "validity: proven"}},
		{"let view", 4, []string{"**Borrow**", "referent: `point`", "holder: `view`"}},
		{"ref view.y", 0, []string{"holder: `again`", "reborrow of `view`"}},
		{"ref mut point", 0, []string{"mutable borrow (`ref mut`)", "authority: exclusive"}},
		{"Read(point", 5, []string{"implicit call-site borrow", "ends: after the call"}},
	}
	for _, test := range cases {
		idx := strings.Index(text, test.marker)
		if idx < 0 {
			t.Fatalf("marker %q missing", test.marker)
		}
		hover, ok := hoverForSource(uri, text, offsetPosition(text, idx+test.skip))
		if !ok {
			t.Fatalf("%q: no hover", test.marker)
		}
		for _, want := range test.want {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("%q: hover lacks %q", test.marker, want)
			}
		}
	}
}

// A holder ended by its last required use reports one live-range end, never
// an additional scope-exit end without a position.
func TestBorrowHoverReportsOneLiveRangeEnd(t *testing.T) {
	text := "module main\n\nfn Main() int {\n    let value := 1\n    let view := ref value\n    let copy := *view\n    let other := 2\n    return copy + other\n}\n"
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	hover, _ := hoverForSource(uri, text, offsetPosition(text, strings.Index(text, "ref value")))
	if !strings.Contains(hover.Contents.Value, "ends: after its last required use, before 7:5") || strings.Contains(hover.Contents.Value, "0:0") {
		t.Fatalf("live-range end:\n%s", hover.Contents.Value)
	}
}
