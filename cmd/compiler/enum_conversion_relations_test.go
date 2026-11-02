package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// A nonexistent string-enum/integer relation is reported at source checking,
// with the canonical mandatory conversion ID, before any IR output is written.
// Rules: rules/types/types.md — Explicit conversions (MD-012 §4.1);
// rules/declarations/enums.md — §12; rules/tooling/diagnostics.md — §§14–15.
func TestEnumCheckedAnalysisRelationsCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_conversion_relations/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if name == "valid" {
			if code != 0 {
				t.Fatal(code, stdout, stderr)
			}
			continue
		}
		doc := decodeOccurrenceDocument(t, stderr)
		if code != 3 || doc.Summary.Errors != 6 || len(doc.Occurrences) != 6 {
			t.Fatal(code, stdout, stderr)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || *d.ID != diagnostics.ConversionUnavailable || len(d.Help) == 0 || d.Unregistered {
				t.Fatal(d)
			}
		}
		artifact := filepath.Join(root, "invalid.ir")
		stdout, stderr, code = runCLIForDiagnostics(t, "emit-ir", path, "-o", artifact)
		if code != 3 {
			t.Fatal(code, stdout, stderr)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatalf("invalid source emitted artifact: %v", err)
		}
	}
}
