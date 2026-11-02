package main

import (
	"os"
	"path/filepath"
	"testing"
)

// CLI check preserves ordinary Result/try rules for enum narrowing, including
// must-use and argument/statement boundaries; it does not impose eager try on
// valid Result carriers, match, projections or explicit discard.
// Rules: rules/errors/errorhandling.md — §§7,9,13,14,26;
// rules/declarations/enums.md — §12.1.
func TestEnumConversionOrdinaryTryCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_conversion_try/" + name + ".sec")
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
		document := decodeOccurrenceDocument(t, stderr)
		if code != 3 || document.Summary.Errors != 7 {
			t.Fatal(code, stdout, stderr)
		}
	}
}
