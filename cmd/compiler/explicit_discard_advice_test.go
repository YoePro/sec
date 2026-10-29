package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestExplicitDiscardAdviceCLI exercises canonical IDs and independent project
// policy in JSON; promotion changes exit status without changing discard rules.
// Rules: rules/control-flow/discard.md — §§13,27,37–38; diagnostics.md — §18.
func TestExplicitDiscardAdviceCLI(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/explicit_discard_advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"default", "off", "warning", "error", "mixed"} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
			t.Fatal(err)
		}
		manifest := "[project]\nname = \"explicit-discard\"\n[version]\nmajor=0\nminor=1\nrevision=0\nbuild=0\nformat=\"[major].[minor].[revision].[build]\"\n"
		useless, void := level, level
		if level == "mixed" {
			useless, void = "off", "warning"
		}
		if level != "default" {
			manifest += "[diagnostics.rules]\n\"ownership.useless-explicit-discard\"=\"" + useless + "\"\n\"ownership.redundant-void-discard\"=\"" + void + "\"\n"
		}
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "main.sec")
		if err := os.WriteFile(file, source, 0600); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runCLIForDiagnostics(t, "check", file, "--diagnostic-format=json")
		doc := decodeOccurrenceDocument(t, stderr)
		want := strings.Count(string(source), "// useless") + strings.Count(string(source), "// void")
		wantCode := 0
		severity := diagnostics.SeverityInformation
		if level == "off" {
			want = 0
		}
		if level == "mixed" {
			want = strings.Count(string(source), "// void")
		}
		if level == "warning" || level == "mixed" {
			severity = diagnostics.SeverityWarning
		}
		if level == "error" {
			severity = diagnostics.SeverityError
			wantCode = 3
		}
		if code != wantCode || len(doc.Occurrences) != want {
			t.Fatal(level, code, stderr)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || d.Name == nil || d.Unregistered || d.Primary == nil || d.Severity != severity || len(d.Help) == 0 {
				t.Fatal(d)
			}
			switch *d.ID {
			case diagnostics.UselessExplicitDiscard:
				if *d.Name != "ownership.useless-explicit-discard" || level == "mixed" {
					t.Fatal(d)
				}
			case diagnostics.RedundantVoidDiscard:
				if *d.Name != "ownership.redundant-void-discard" {
					t.Fatal(d)
				}
			default:
				t.Fatal(d)
			}
		}
	}
}
