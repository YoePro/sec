package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Formatting edits use the live snapshot, retain unaffected text and use
// UTF-16 positions after non-BMP source characters. Applying the edit gives
// the golden canonical bytes and a second request returns no edits.
// Rules: rules/tooling/formatter.md — § 28(1)–(2), § 29(1)–(4), (7).
func TestFormatDocumentMinimalEdit(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/minimal_edit.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/minimal_edit.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for name, newline := range map[string]string{"LF": "\n", "CRLF": "\r\n"} {
		for _, open := range []bool{true, false} {
			t.Run(name+map[bool]string{true: "/snapshot", false: "/disk"}[open], func(t *testing.T) {
				text := strings.ReplaceAll(string(input), "\n", newline)
				// The shared formatter canonicalizes CRLF to LF; the edit
				// must reproduce those exact bytes rather than preserve CRLF.
				want := string(golden)
				path := filepath.Join(t.TempDir(), "main.sec")
				uri := uriFromPath(path)
				s := &server{documentSnapshots: lspserver.NewDocuments()}
				if open {
					s.documentSnapshots.Open(uri, 1, text)
				} else if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
				edits, err := s.formatDocument(uri)
				if err != nil || len(edits) != 1 {
					t.Fatalf("formatDocument = %#v, %v", edits, err)
				}
				edit := edits[0]
				if newline == "\n" && (edit.Range.Start != (position{Line: 4, Character: 15}) ||
					edit.Range.End != (position{Line: 4, Character: 16}) || edit.NewText != " + ") {
					t.Fatalf("edit includes unchanged text or wrong UTF-16 position: %#v", edit)
				}
				if newline == "\r\n" && (edit.Range.Start != (position{Line: 0, Character: 17}) ||
					edit.Range.End != (position{Line: 6, Character: 0})) {
					t.Fatalf("line-ending edit splits a CRLF or includes unchanged prefix: %#v", edit)
				}
				start := lineCharToOffset(text, edit.Range.Start.Line, edit.Range.Start.Character)
				end := lineCharToOffset(text, edit.Range.End.Line, edit.Range.End.Character)
				got := text[:start] + edit.NewText + text[end:]
				if got != want {
					t.Fatalf("applied edit = %q; want %q", got, want)
				}
				s.documentSnapshots.Open(uri, 2, got)
				if edits, err := s.formatDocument(uri); err != nil || len(edits) != 0 {
					t.Fatalf("second formatting = %#v, %v", edits, err)
				}
			})
		}
	}
}
