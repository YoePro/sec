package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The CLI uses the canonical enum narrowing channel for Result/try and rejects
// unmapped legacy ArithmeticError carriers, without requiring executable lowering.
// Rules: rules/types/types.md — "Explicit conversions" (MD-012, MD-104);
// rules/declarations/enums.md — §12.1.
func TestEnumConversionErrorChannelCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_conversion_errors/" + name + ".sec")
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
		if code != 3 || document.Summary.Errors != 4 || !strings.Contains(stderr, "Result[uint8, ConversionError]") || !strings.Contains(stderr, "300 overflows uint8") {
			t.Fatal(code, stdout, stderr)
		}
	}
}
