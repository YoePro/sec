package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLI checking uses known enum values through constructors and immutable
// bindings, retaining compile-time overflow instead of a runtime Result.
// Rules: rules/declarations/enums.md — §12.1; rules/types/types.md — Exact integer conversion proof.
func TestKnownEnumValuesCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_known_values/" + name + ".sec")
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
		if code != 3 || doc.Summary.Errors != 4 || !strings.Contains(stderr, "overflows uint8") || !strings.Contains(stderr, "overflows int8") {
			t.Fatal(code, stdout, stderr)
		}
	}
}
