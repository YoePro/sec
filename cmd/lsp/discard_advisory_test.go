package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRedundantDiscardAdvisoryLSP reads project policy for unsaved source and
// preserves canonical severity, explanation and earlier ownership source links.
// Rules: rules/control-flow/discard.md — §§5,38;
// rules/memory/destruction.md — §33.2(4); rules/tooling/lsp.md — "Diagnostics".
func TestRedundantDiscardAdvisoryLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/redundant_discard_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(dir, "main.sec"))
	for _, level := range []string{"info", "off", "warning", "error", "info"} {
		manifest := "[diagnostics.rules]\n\"ownership.redundant-discard\" = \"" + level + "\"\n"
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(source))
		want, severity := 3, 3
		if level == "off" {
			want = 0
		}
		if level == "warning" {
			severity = 2
		}
		if level == "error" {
			severity = 1
		}
		if len(items) != want {
			t.Fatal(level, items)
		}
		for _, d := range items {
			if d.Code != "A2006" || d.Severity != severity || len(d.RelatedInformation) == 0 || !strings.Contains(d.Message, "legal no-op") {
				t.Fatal(level, d)
			}
		}
	}
}
