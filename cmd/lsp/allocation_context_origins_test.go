package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Operation and callable hovers retain origin through moves and synchronous
// calls. New unsaved snapshots refresh source locations; ambient selection is
// shown through profile facts, never an invented source or backing authority.
// Rules: rules/memory/allocation.md §§5,6,29;
// rules/memory/arena.md §§4.2,4.3,67(4); rules/tooling/lsp.md — Hover, snapshots.
func TestAllocationContextOriginsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/allocation_context_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, shift := range []int{0, 2} {
		text := strings.Repeat("\n", shift) + string(data)
		for _, test := range []struct {
			marker string
			skip   int
			origin string
			line   int
		}{
			{"mem.New[byte]", 4, "allocation context origin", 11},
			{"moved.Alloc[byte]", 6, "allocation context origin", 11},
			{"fn OtherDomain", 3, "allocation context origin", 25},
			{"fn Combined", 3, "allocation context origin", 11},
			{"Arena.WithCapacity", 6, "created domain origin", 30},
		} {
			hover, ok := hoverForSource(uri, text, offsetPosition(text, strings.Index(text, test.marker)+test.skip))
			want := fmt.Sprintf("%s: %s:%d:", test.origin, pathFromURI(uri), test.line+shift)
			if !ok || !strings.Contains(hover.Contents.Value, want) {
				t.Fatal("missing origin", want, hover)
			}
		}
		hover, _ := hoverForSource(uri, text, offsetPosition(text, strings.Index(text, "fn Render")+3))
		if !strings.Contains(hover.Contents.Value, "origin Arena; domain identity unresolved") || strings.Contains(hover.Contents.Value, "allocation context origin:") {
			t.Fatal("fabricated implicit location", hover)
		}
	}
}
