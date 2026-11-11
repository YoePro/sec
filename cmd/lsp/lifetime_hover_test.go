package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Hover over a reference holder's declaration shows the returned-reference
// source, the Arena generation it depends on with the invalidating Reset,
// and the end of the retained borrow, from Sema facts only.
// Rules: rules/memory/lifetime_analysis.md — §§ 2, 6, 12; rules/memory/arena.md — § 44;
// rules/tooling/lsp.md — "Hover".
func TestLifetimeHover(t *testing.T) {
	text := `module main

type Pair struct {
    a: int,
    b: int,
}

fn First(pair: ref Pair) ref int {
    return ref pair.a
}

fn Main() int {
    let pair := Pair { a: 1, b: 2 }
    let first := First(ref pair)
    let copy := *first
    let other := 3
    return copy + other
}

fn Scratch() Result[int, AllocationError] {
    let mut arena := try Arena.WithCapacity(64u)
    let slot := try arena.New[int]()
    *slot = 4
    let value := *slot
    arena.Reset()
    return Ok(value)
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	cases := []struct {
		marker string
		want   []string
	}{
		{"first := First", []string{"**Lifetime**", "returned reference: `First` returns a reference into `pair.a`", "retained by: `first`; the dependency ends after its last required use, before 16:5"}},
		{"slot := try", []string{"generation: valid in Arena epoch 0", "invalidation: ends the reference's validity at reset at 25:"}},
	}
	for _, test := range cases {
		idx := strings.Index(text, test.marker)
		if idx < 0 {
			t.Fatalf("marker %q missing", test.marker)
		}
		hover, ok := hoverForSource(uri, text, offsetPosition(text, idx))
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

// Function hover presents the returned-reference origin summary with source
// parameter names.
// Rules: rules/memory/references.md — returned-reference summaries; rules/tooling/lsp.md — "Hover".
func TestReturnedReferenceSummaryHover(t *testing.T) {
	text := `module main

type Pair struct {
    a: int,
    b: int,
}

fn First(pair: ref Pair) ref int {
    return ref pair.a
}

fn Pick(left: ref int, right: ref int, flag: bool) ref int {
    if flag {
        return left
    }
    return right
}

fn Main() int {
    let pair := Pair { a: 1, b: 2 }
    let other := 3
    return *First(ref pair) + *Pick(ref pair.b, ref other, true)
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for marker, want := range map[string]string{
		"fn First": "Returned reference: into `pair.a`",
		"fn Pick":  "Returned reference: into `left` or `right`",
	} {
		hover, _ := hoverForSource(uri, text, offsetPosition(text, strings.Index(text, marker)+3))
		if !strings.Contains(hover.Contents.Value, want) {
			t.Errorf("%s hover lacks %q:\n%s", marker, want, hover.Contents.Value)
		}
	}
	hover, _ := hoverForSource(uri, text, offsetPosition(text, strings.Index(text, "fn Main")+3))
	if strings.Contains(hover.Contents.Value, "Returned reference") {
		t.Errorf("non-reference function got a summary:\n%s", hover.Contents.Value)
	}
}
