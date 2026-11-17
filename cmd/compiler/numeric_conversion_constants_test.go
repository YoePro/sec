package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNumericConversionConstantFailuresCLI verifies the frontend boundary:
// known-invalid conversions fail during check and never require native lowering.
// Rules: rules/types/types.md — §25(9–11); rules/tooling/diagnostics.md — Compiler diagnostics.
func TestNumericConversionConstantFailuresCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, valid := range []bool{true, false} {
		name := "numeric_conversion_constants_valid.sec"
		if !valid {
			name = "numeric_conversion_constants_invalid.sec"
		}
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sema", name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if valid {
			if code != 0 {
				t.Fatal(code, stdout, stderr)
			}
			continue
		}
		document := decodeOccurrenceDocument(t, stderr)
		if code != 3 || document.Summary.Errors != 12 || !strings.Contains(stderr, "constant conversion from decimal to int32") || !strings.Contains(stderr, "ConversionError.NonFinite") {
			t.Fatal(code, stdout, stderr)
		}
	}
}
