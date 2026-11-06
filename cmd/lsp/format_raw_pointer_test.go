package main

import (
	"os"
	"path/filepath"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Applying the document edit preserves RawPtr operations and unsafe boundaries
// exactly as sec fmt, with no second edit for the formatted snapshot.
// Rules: rules/memory/raw_pointers.md §36; rules/tooling/formatter.md §§17,19(11–14),28,29.
func TestLSPFormatRawPointerOperations(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/raw_pointer_operations.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/raw_pointer_operations.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	s := &server{documentSnapshots: lspserver.NewDocuments()}
	s.documentSnapshots.Open(uri, 1, string(input))
	edits, err := s.formatDocument(uri)
	if err != nil || len(edits) != 1 {
		t.Fatalf("formatDocument: %v %v", edits, err)
	}
	edit := edits[0]
	start := lineCharToOffset(string(input), edit.Range.Start.Line, edit.Range.Start.Character)
	end := lineCharToOffset(string(input), edit.Range.End.Line, edit.Range.End.Character)
	got := string(input[:start]) + edit.NewText + string(input[end:])
	if got != string(want) {
		t.Fatalf("LSP differs from CLI golden:\n%s", got)
	}
	s.documentSnapshots.Open(uri, 2, got)
	if edits, err := s.formatDocument(uri); err != nil || len(edits) != 0 {
		t.Fatalf("second format: %v %v", edits, err)
	}
}
