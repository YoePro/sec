package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestImplicitEmptyCatchAllCLI preserves JSON identity/location, the human
// warning and project policy; only promotion rejects the otherwise legal source.
// Rules: rules/declarations/unions.md — §10.4;
// rules/tooling/diagnostics.md — §§18,24.
func TestImplicitEmptyCatchAllCLI(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/union_empty_catch_all.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"default", "off", "info", "warning", "error"} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
			t.Fatal(err)
		}
		manifest := "[project]\nname = \"empty-advisory\"\n[version]\nmajor = 0\nminor = 1\nrevision = 0\nbuild = 0\nformat = \"[major].[minor].[revision].[build]\"\n"
		if level != "default" {
			manifest += "[diagnostics.rules]\n\"union.implicit-empty-catch-all\" = \"" + level + "\"\n"
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
		wantCode, count := 0, 3
		severity := diagnostics.SeverityWarning
		if level == "off" {
			count = 0
		}
		if level == "info" {
			severity = diagnostics.SeverityInformation
		}
		if level == "error" {
			wantCode = 3
			severity = diagnostics.SeverityError
		}
		if code != wantCode || len(doc.Occurrences) != count {
			t.Fatal(level, code, stderr)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || *d.ID != diagnostics.ImplicitEmptyCatchAll || d.Name == nil || *d.Name != "union.implicit-empty-catch-all" || d.Unregistered || d.Primary == nil || d.Severity != severity || len(d.Help) == 0 {
				t.Fatal(d)
			}
		}
		if level == "default" {
			_, human, humanCode := runCLIForDiagnostics(t, "check", file)
			if humanCode != 0 || !strings.Contains(human, "Warning:") || !strings.Contains(human, "possible empty state of state") {
				t.Fatal(humanCode, human)
			}
		}
	}
}
