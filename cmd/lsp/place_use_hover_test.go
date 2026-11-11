package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Hover over a later read shows where the value was moved in from, the partial
// state of the enclosing owner and a source-order consequence preview of a
// move at that read, including the loop repetition note.
// Rules: rules/memory/copy_move.md — § 29(1)–(2), (4); rules/tooling/lsp.md — "Hover".
func TestPlaceUseHover(t *testing.T) {
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

fn Look(r: ref Res) int {
    return r.id
}

fn Main() int {
    let r := Res { id: 1 }
    let moved :<- r
    let one := Look(ref moved)
    let two := Look(ref moved)
    let p := Pair { a: Res { id: 2 }, b: Res { id: 3 } }
    let first :<- p.a
    let rest := p.b.id
    let mut total := 0
    let mut i := 0
    while i < 2 {
        total = total + Look(ref first)
        i = i + 1
    }
    return one + two + rest + total
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	cases := []struct {
		marker string
		skip   int
		want   []string
		reject []string
	}{
		{"Look(ref moved)\n    let two", 9, []string{"**Availability**", "`moved` is available", "value origin: moved in from `r` at 23:", "if moved here: `moved` becomes unavailable for 1 later use (25:"}, nil},
		{"two := Look(ref moved", 20, []string{"value origin: moved in from `r`", "no later use of `moved` follows"}, []string{"enclosing loop"}},
		{"p.b.id", 4, []string{"owner `p` is partially available (`p.a` moved at 27:"}, nil},
		{"Look(ref first", 9, []string{"value origin: moved in from `p.a`", "the enclosing loop repeats this read"}, nil},
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
				t.Errorf("%q: hover lacks %q:\n%s", test.marker, want, hover.Contents.Value)
			}
		}
		for _, reject := range test.reject {
			if strings.Contains(hover.Contents.Value, reject) {
				t.Errorf("%q: hover unexpectedly has %q:\n%s", test.marker, reject, hover.Contents.Value)
			}
		}
	}
	// Trivially copyable reads get availability but no move preview.
	idx := strings.Index(text, "i < 2")
	hover, _ := hoverForSource(uri, text, offsetPosition(text, idx))
	if !strings.Contains(hover.Contents.Value, "`i` is available") || strings.Contains(hover.Contents.Value, "if moved here") {
		t.Errorf("trivial read hover:\n%s", hover.Contents.Value)
	}
}
