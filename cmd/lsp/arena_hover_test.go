package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Hover over an Arena declaration or Arena operation receiver shows the
// domain, its creation with backing, provider and capacity, the represented
// epoch at that point and the allocation profile; an Arena parameter's domain
// is caller-provided and gets no invented backing.
// Rules: rules/memory/arena.md — §§ 4.2, 10–12, 44, 77; rules/tooling/lsp.md — "Hover".
func TestArenaHover(t *testing.T) {
	text := `module main

fn Owned() Result[void, AllocationError] {
    let mut arena := try Arena.WithCapacity(64u)
    let first := try arena.New[byte]()
    discard first
    arena.Reset()
    let second := try arena.New[byte]()
    discard second
    return Ok()
}

fn Borrowed() Result[void, AllocationError] {
    let mut buffer: byte[32] := [0u8; 32]
    let mut scratch := Arena.FromBuffer(buffer)
    let value := try scratch.New[byte]()
    discard value
    return Ok()
}

fn Fill(pool: ref mut Arena) Result[void, AllocationError] {
    let value := try pool.New[byte]()
    discard value
    return Ok()
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	cases := []struct {
		marker string
		skip   int
		want   []string
	}{
		{"arena := try", 0, []string{"**Arena**", "created by: `Arena.WithCapacity` at 4:", "backing: owned backing", "provider: the selected target/profile backing provider", "capacity: `64u` = 64 bytes requested", "epoch: 0", "profile: "}},
		{"arena.New[byte]()\n    discard first", 0, []string{"epoch: 0"}},
		{"arena.New[byte]()\n    discard second", 0, []string{"epoch: 1 (Reset calls"}},
		{"scratch :=", 0, []string{"created by: `Arena.FromBuffer`", "backing: borrowed `buffer`", "no provider"}},
		{"pool.New", 0, []string{"created by: the caller"}},
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
