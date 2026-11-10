package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAsmVolatileSyntaxCLI rejects C-style asm modifiers before semantic analysis or IR emission.
// Rules: rules/platform/inline_assembly.md — §46.
func TestAsmVolatileSyntaxCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/parser/asm_volatile_identifier_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/parser/asm_volatile_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 2 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 3 {
			t.Fatal(output)
		}
		if !strings.Contains(output, "P2012") || !strings.Contains(output, "parser.reserved-syntax") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid volatile asm emitted an artifact", err)
		}
	}
}
