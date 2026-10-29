package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImplicitDiscardedResultLSP uses canonical Sema facts and project policy
// for unsaved source, retaining the same A3001 code, range and explanation.
// Rules: rules/control-flow/discard.md — §§12–13,37–38;
// rules/tooling/lsp.md — "Diagnostics"; rules/tooling/diagnostics.md — §18.
func TestImplicitDiscardedResultLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/implicit_discard.sec")
	if err != nil {
		t.Fatal(err)
	}
	invalidSource, err := os.ReadFile("../../testdata/tooling/implicit_discard_invalid.sec")
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
			manifest = "[diagnostics.rules]\n\"ownership.implicit-discarded-result\" = \"" + level + "\"\n"
		}
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(source))
		count, severity := strings.Count(string(source), "// advisory"), 3
		if level == "off" {
			count = 0
		}
		if level == "warning" {
			severity = 2
		}
		if level == "error" {
			severity = 1
		}
		if len(items) != count {
			t.Fatal(level, items)
		}
		for _, d := range items {
			if d.Code != "A3001" || d.Severity != severity || d.Range.End.Character <= d.Range.Start.Character || !strings.Contains(d.Message, "implicitly discarded") || !strings.Contains(d.Message, "discard` before the call") {
				t.Fatal(level, d)
			}
		}
		mustUse := 0
		for _, diagnostic := range analyze(uri, string(invalidSource)) {
			if diagnostic.Code == "S1005" && diagnostic.Severity == 1 {
				mustUse++
			}
			if diagnostic.Code == "A3001" {
				t.Fatal("invalid call got advisory", diagnostic)
			}
		}
		if mustUse != 2 {
			t.Fatal("mandatory must-use diagnostics changed", level, mustUse)
		}

	}
}
