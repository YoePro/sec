package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRedundantDiscardAdvisoryCLI uses real project policy for legal repeated
// discards. Only policy promotion rejects the source, and no source is edited.
// Rules: rules/control-flow/discard.md — §§5,38;
// rules/tooling/diagnostics.md — §18.
func TestRedundantDiscardAdvisoryCLI(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/redundant_discard_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"off", "info", "warning", "error"} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
			t.Fatal(err)
		}
		manifest := "[project]\nname = \"discard\"\n[version]\nmajor = 0\nminor = 1\nrevision = 0\nbuild = 0\nformat = \"[major].[minor].[revision].[build]\"\n[diagnostics.rules]\n\"ownership.redundant-discard\" = \"" + level + "\"\n"
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "main.sec")
		if err := os.WriteFile(file, source, 0600); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runCLIForDiagnostics(t, "check", file, "--diagnostic-format=json")
		doc := decodeOccurrenceDocument(t, stderr)
		wantCode, count := 0, 3
		if level == "off" {
			count = 0
		}
		if level == "error" {
			wantCode = 3
		}
		if code != wantCode || len(doc.Occurrences) != count {
			t.Fatal(level, code, stderr)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || *d.ID != "A2006" || d.Primary == nil || len(d.Related) == 0 || d.Unregistered {
				t.Fatal(d)
			}
		}
		after, err := os.ReadFile(file)
		if err != nil || string(after) != string(source) {
			t.Fatal("check edited source", err)
		}
	}
}
