package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestImplicitDiscardedResultCLI preserves JSON identity/location, the human
// advisory and project policy; only promotion rejects the otherwise legal source.
// Rules: rules/control-flow/discard.md — §§12–13,37–38;
// rules/tooling/diagnostics.md — §§18,24.
func TestImplicitDiscardedResultCLI(t *testing.T) {
	source, err := os.ReadFile("../../testdata/tooling/implicit_discard.sec")
	if err != nil {
		t.Fatal(err)
	}
	invalidSource, err := os.ReadFile("../../testdata/tooling/implicit_discard_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"default", "off", "info", "warning", "error"} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
			t.Fatal(err)
		}
		manifest := "[project]\nname = \"implicit-discard-advisory\"\n[version]\nmajor = 0\nminor = 1\nrevision = 0\nbuild = 0\nformat = \"[major].[minor].[revision].[build]\"\n"
		if level != "default" {
			manifest += "[diagnostics.rules]\n\"ownership.implicit-discarded-result\" = \"" + level + "\"\n"
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
		wantCode, count := 0, strings.Count(string(source), "// advisory")
		severity := diagnostics.SeverityInformation
		if level == "off" {
			count = 0
		}
		if level == "warning" {
			severity = diagnostics.SeverityWarning
		}
		if level == "error" {
			wantCode = 3
			severity = diagnostics.SeverityError
		}
		if code != wantCode || len(doc.Occurrences) != count {
			t.Fatal(level, code, stderr)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || *d.ID != diagnostics.ImplicitDiscardedResult || d.Name == nil || *d.Name != "ownership.implicit-discarded-result" || d.Unregistered || d.Primary == nil || d.Severity != severity || len(d.Help) == 0 {
				t.Fatal(d)
			}
		}
		if level == "default" {
			_, human, humanCode := runCLIForDiagnostics(t, "check", file)
			if humanCode != 0 || !strings.Contains(human, "implicitly discarded") {
				t.Fatal(humanCode, human)
			}
		}
		if err := os.WriteFile(file, invalidSource, 0600); err != nil {
			t.Fatal(err)
		}
		_, stderr, code = runCLIForDiagnostics(t, "check", file, "--diagnostic-format=json")
		invalid := decodeOccurrenceDocument(t, stderr)
		mustUse := 0
		for _, occurrence := range invalid.Occurrences {
			if occurrence.ID != nil && *occurrence.ID == diagnostics.UnhandledMustUseResult {
				mustUse++
			}
			if occurrence.ID != nil && *occurrence.ID == diagnostics.ImplicitDiscardedResult {
				t.Fatal("invalid call got advisory", occurrence)
			}
		}
		if code != 3 || mustUse != 2 {
			t.Fatal(level, code, stderr)
		}

	}
}
