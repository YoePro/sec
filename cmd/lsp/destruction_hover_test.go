package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Hover over an owned binding's declaration shows its destruction
// classification and custom free, the cleanup owner, partial terminal uses,
// pending deferred uses and the expected terminal action.
// Rules: rules/memory/destruction.md — §§ 3, 6.1, 9–12, 33; rules/tooling/lsp.md — "Hover".
func TestDestructionHover(t *testing.T) {
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

fn Main() int {
    let kept := Res { id: 1 }
    let gone := Res { id: 2 }
    let pair := Pair { a: Res { id: 3 }, b: Res { id: 4 } }
    defer {
        let seen := kept.id
    }
    let first :<- pair.a
    let n := Consume(<-gone)
    return n + first.id
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	cases := []struct {
		marker string
		want   []string
	}{
		{"kept :=", []string{"**Destruction**", "struct; runs its custom `free` first", "cleanup owner: `kept` (declaration); the obligation starts after successful initialization", "pending deferred use at 26:", "expected terminal action: destroyed at the end of its scope"}},
		{"gone :=", []string{"expected terminal action: ownership ends where `gone` is moved at 29:"}},
		{"pair :=", []string{"classification: struct; destroys owned fields in reverse declaration order: `b`, `a`", "partial state: `pair.a` moved at 28:", "destroyed at the end of its scope"}},
		{"r: Res", []string{"cleanup owner: `r` (parameter); the obligation starts on entry to the callable"}},
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
	// Trivially destructible bindings have no destruction section.
	idx := strings.Index(text, "n := Consume")
	if hover, _ := hoverForSource(uri, text, offsetPosition(text, idx)); strings.Contains(hover.Contents.Value, "**Destruction**") {
		t.Errorf("trivial binding got destruction hover:\n%s", hover.Contents.Value)
	}
}
