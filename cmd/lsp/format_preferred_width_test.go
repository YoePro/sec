package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Actual formatting requests consume project width and optional observed
// viewport columns. Personal tab/space preferences cannot override project
// style; absence of viewport metadata uses exactly the CLI fallback.
// Rules: rules/tooling/formatter.md — §4(9–23),§16(1–2),§28;
// rules/tooling/lsp.md — Shared formatter.
func TestLSPPreferredWidthProtocol(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/preferred_width.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		mode     string
		fixed    uint64
		observed uint64
		narrow   bool
	}{
		{"fixed", 64, 200, true},
		{"fixed", 120, 1, false},
		{"adaptive", 1, 0, true},
		{"adaptive", 1, 1, true},
		{"adaptive", 1, 120, false},
		{"adaptive", 1, 9999, false},
	} {
		root := t.TempDir()
		manifestPath := filepath.Join(root, ".sec", "sec.toml")
		if err := os.MkdirAll(filepath.Dir(manifestPath), 0700); err != nil {
			t.Fatal(err)
		}
		manifest := fmt.Sprintf("[format]\nwidth_mode = %q\nline_width = %d\nadaptive_min_width = 64\nadaptive_max_width = 120\nadaptive_fallback_width = 64\n", test.mode, test.fixed)
		if err := os.WriteFile(manifestPath, []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		name := "preferred_width.expected.sec"
		if test.narrow {
			name = "preferred_width.narrow.expected.sec"
		}
		want, err := os.ReadFile("../../testdata/formatter/" + name)
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(root, "source.sec"))
		s := &server{documentSnapshots: lspserver.NewDocuments()}
		s.documentSnapshots.Open(uri, 1, string(input))
		for _, invalid := range []any{-1, 1.5, "auto", true} {
			response := lspTestRequest(t, s, "textDocument/formatting", map[string]any{
				"textDocument": map[string]any{"uri": uri}, "options": map[string]any{"secPresentationWidth": invalid},
			})
			if response.Error == nil || response.Error.Code != -32602 {
				t.Fatal("invalid viewport did not return InvalidParams", response)
			}
		}
		options := map[string]any{"tabSize": 9, "insertSpaces": false}
		if test.observed != 0 {
			options["secPresentationWidth"] = test.observed
		}
		params := map[string]any{"textDocument": map[string]any{"uri": uri}, "options": options}
		edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/formatting", params))
		if len(edits) != 1 {
			t.Fatal("expected one minimal formatting edit", edits)
		}
		edit := edits[0]
		start := lineCharToOffset(string(input), edit.Range.Start.Line, edit.Range.Start.Character)
		end := lineCharToOffset(string(input), edit.Range.End.Line, edit.Range.End.Character)
		got := string(input[:start]) + edit.NewText + string(input[end:])
		if got != string(want) {
			t.Fatalf("%s width %d viewport %d:\n%s\nwant:\n%s", test.mode, test.fixed, test.observed, got, want)
		}
		s.documentSnapshots.Open(uri, 2, got)
		if edits := lspTestDecode[[]textEdit](t, lspTestRequest(t, s, "textDocument/formatting", params)); len(edits) != 0 {
			t.Fatal("second request changed canonical output", edits)
		}
	}
}
