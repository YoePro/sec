package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPropertyReceiverCLI preserves inferred receiver authority in compiler
// diagnostics and rejects invalid property access before producing IR.
// Rules: rules/declarations/properties.md — §8; rules/memory/borrowing.md — §7.
func TestPropertyReceiverCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/property_receivers/valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "property.ir")
		args := []string{command, "../../testdata/sema/property_receivers/authority_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 5 {
			t.Fatal(output)
		}
		if strings.Count(output, `"id": "S1219"`) != 3 || !strings.Contains(output, "mutable receiver") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid receiver access produced IR", err)
		}
	}
}
