package main

import (
	"os"
	"path/filepath"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Applying the actual metadata document edit produces the CLI golden and
// preserves legacy values; a second request on that snapshot has no edit.
// Rules: rules/types/units.md — "Formatter requirements";
// rules/tooling/formatter.md §§22(11–13),28,29.
func TestLSPFormatUnitMetadataSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/unit_metadata_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/unit_metadata_spacing.expected.sec")
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
