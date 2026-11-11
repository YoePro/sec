package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Invalid lifecycle calls retain normal member hover and add the same Sema
// Invalid proof, help, dependency and retention chain as mandatory diagnostics.
// Fresh unsaved source coordinates must replace the prior snapshot's locations.
// Rules: rules/memory/arena.md §§36,44,51,119–120;
// rules/tooling/lsp.md — Hover, shared diagnostic model.
func TestArenaInvalidationBlockerHover(t *testing.T) {
	for _, test := range []struct{ file, marker, retainer string }{
		{"arena_invalidation_diagnostics_invalid.sec", "mem.Reset()", "deferred use keeps the Arena dependency live"},
		{"arena_invalidation_diagnostics_invalid.sec", "moved.Release()", "deferred use keeps the Arena dependency live"},
		{"arena_nested_invalid.sec", "parent.Reset()", "child Arena retains parent backing here"},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
		at := strings.Index(text, test.marker) + strings.Index(test.marker, ".") + 1
		hover, ok := hoverForSource(uri, text, offsetPosition(text, at))
		if !ok || !strings.Contains(hover.Contents.Value, "**Arena lifecycle blocked**") || !strings.Contains(hover.Contents.Value, "Invalid:") || !strings.Contains(hover.Contents.Value, "live Arena dependency") || !strings.Contains(hover.Contents.Value, test.retainer) || !strings.Contains(hover.Contents.Value, "Arena") {
			t.Fatal(test, hover)
		}
		if strings.Contains(hover.Contents.Value, "$defer") || strings.Contains(hover.Contents.Value, "$arena-domain") {
			t.Fatal("internal holder ID leaked", hover)
		}
		// The real member token remains the hover range even for rejected calls.
		if hover.Range.Start != offsetPosition(text, at) {
			t.Fatal("wrong hover span", hover.Range)
		}
		if test.marker == "mem.Reset()" {
			shifted := "\n\n" + text
			next, found := hoverForSource(uri, shifted, offsetPosition(shifted, at+2))
			if !found || next.Range.Start.Line != hover.Range.Start.Line+2 || next.Contents.Value == hover.Contents.Value {
				t.Fatal("stale unsaved blocker", next)
			}
		}
	}
}
