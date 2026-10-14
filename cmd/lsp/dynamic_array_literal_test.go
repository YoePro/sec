package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Shared Sema diagnostics for forbidden owner literals must retain their ID,
// source location and actionable explanation in an unsaved LSP document.
// Rules: rules/collections/collections.md §5.6a;
// rules/tooling/lsp.md — Diagnostics.
func TestAnalyzeDynamicArrayLiteralDiagnostic(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	text := "module main\nfn Main() void {\n    let values: int[] := [1, 2]\n}\n"
	items := analyze(uri, text, sourceOverlay{Sources: map[string]string{pathFromURI(uri): text}})
	if len(items) != 1 || items[0].Code != "S1155" || items[0].Range.Start.Line != 2 || items[0].Range.Start.Character != 25 || !strings.Contains(items[0].Message, "Append") {
		t.Fatalf("LSP diagnostics = %+v", items)
	}
}
