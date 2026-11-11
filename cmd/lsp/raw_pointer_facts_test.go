package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Hover shares source-local compiler proofs and does not turn a raw address
// or unsafe context into spatial/lifetime validity.
// Rules: rules/memory/raw_pointers.md §35(4–6); rules/tooling/lsp.md — Hover.
func TestRawPointerFactHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_facts.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, tc := range []struct{ use, want, absent string }{
		{"let empty:", "Nullability: proven null", "Known origin:"},
		{"let branch := input", "Nullability: proven null", "Known origin:"},
		{"let outside := input", "Nullability: not established", "proven null"},
		{"let unknown := mutable", "Nullability: not established", "proven null"},
		{"let before := alias", "Known origin: `session`", "Origin no longer holds"},
		{"let invalidated := alias", "Origin no longer holds a value", "proven null"},
	} {
		offset := strings.Index(text, tc.use) + len("let ")
		hover, ok := hoverForSource(uri, text, offsetPosition(text, offset))
		if !ok || !strings.Contains(hover.Contents.Value, tc.want) || strings.Contains(hover.Contents.Value, tc.absent) || !strings.Contains(hover.Contents.Value, "Address space, range, alignment and lifetime: not established") {
			t.Fatalf("%s: %+v", tc.use, hover)
		}
	}
	// The same URI with edited initialization must not retain its old proof.
	edited := strings.Replace(text, "let empty: RawPtr[int] := null", "let empty: RawPtr[int] := input", 1)
	offset := strings.Index(edited, "let empty:") + len("let ")
	hover, ok := hoverForSource(uri, edited, offsetPosition(edited, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "Nullability: not established") || strings.Contains(hover.Contents.Value, "proven null") {
		t.Fatal("stale null proof", hover)
	}
}
