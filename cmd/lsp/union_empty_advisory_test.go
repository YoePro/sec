package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImplicitEmptyCatchAllLSP uses canonical Sema facts and project policy
// for unsaved source, retaining the same A2009 code, range and explanation.
// Rules: rules/declarations/unions.md — §10.4;
// rules/tooling/lsp.md — "Diagnostics"; rules/tooling/diagnostics.md — §18.
func TestImplicitEmptyCatchAllLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/union_empty_catch_all.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(dir, "main.sec"))
	for _, level := range []string{"default", "off", "info", "warning", "error", "off"} {
		manifest := ""
		if level != "default" {
			manifest = "[diagnostics.rules]\n\"union.implicit-empty-catch-all\" = \"" + level + "\"\n"
		}
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(source))
		count, severity := 3, 2
		if level == "off" {
			count = 0
		}
		if level == "info" {
			severity = 3
		}
		if level == "error" {
			severity = 1
		}
		if len(items) != count {
			t.Fatal(level, items)
		}
		for _, d := range items {
			if d.Code != "A2009" || d.Severity != severity || d.Range.End.Character-d.Range.Start.Character != 1 || !strings.Contains(d.Message, "possible empty state of state") || !strings.Contains(d.Message, "empty =>") {
				t.Fatal(level, d)
			}
		}
	}
}
