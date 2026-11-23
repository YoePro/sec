package main

import (
	"os"
	"path/filepath"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Editor formatting shares the CLI attribute golden across all supported
// attachment forms; a second request produces no edit.
// Rules: rules/tooling/formatter.md §§16(14–17),28–29.
func TestLSPFormatGeneralAttributes(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/attributes/general.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/attributes/general.expected.sec")
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
