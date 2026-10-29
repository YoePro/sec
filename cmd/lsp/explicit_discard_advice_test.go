package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExplicitDiscardAdviceLSP retains the canonical source ranges, help and
// independent policies for unsaved source, including changes back to off.
// Rules: rules/control-flow/discard.md — §§13,27,37–38;
// rules/tooling/lsp.md — Diagnostics; diagnostics.md — §18.
func TestExplicitDiscardAdviceLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/explicit_discard_advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(dir, "main.sec"))
	for _, level := range []string{"default", "off", "info", "warning", "error", "mixed", "off"} {
		useless, void := level, level
		if level == "mixed" {
			useless, void = "off", "warning"
		}
		manifest := ""
		if level != "default" {
			manifest = "[diagnostics.rules]\n\"ownership.useless-explicit-discard\"=\"" + useless + "\"\n\"ownership.redundant-void-discard\"=\"" + void + "\"\n"
		}
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(source))
		want := strings.Count(string(source), "// useless") + strings.Count(string(source), "// void")
		severity := 3
		if level == "off" {
			want = 0
		}
		if level == "mixed" {
			want = strings.Count(string(source), "// void")
		}
		if level == "warning" || level == "mixed" {
			severity = 2
		}
		if level == "error" {
			severity = 1
		}
		if len(items) != want {
			t.Fatal(level, items)
		}
		for _, d := range items {
			if (d.Code != "A3002" && d.Code != "A3003") || d.Severity != severity || d.Range.End.Character-d.Range.Start.Character != 7 || !strings.Contains(d.Message, "help:") || (level == "mixed" && d.Code != "A3003") {
				t.Fatal(level, d)
			}
		}
	}
}
