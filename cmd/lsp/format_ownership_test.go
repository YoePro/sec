package main

import (
	"os"
	"path/filepath"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Applying the actual document edit displays the same ownership spelling as
// sec fmt and retains a stable second request on the current snapshot.
// Rules: rules/memory/copy_move.md §29(3); rules/tooling/formatter.md §§21,28,29.
func TestLSPFormatOwnershipMarkers(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/ownership_markers.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/ownership_markers.expected.sec")
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
