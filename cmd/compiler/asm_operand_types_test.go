package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAsmOperandTypesCLI rejects high-level asm register values before an IR
// artifact can be emitted, with registered actionable type diagnostics.
// Rules: rules/platform/inline_assembly.md — §§22,26–27.
func TestAsmOperandTypesCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/asm_operand_types_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/sema/asm_operand_types_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 20 {
			t.Fatal(output)
		}
		if !strings.Contains(output, "S1207") || !strings.Contains(output, "asm.invalid-register-operand-type") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid asm emitted an artifact", err)
		}
	}
}
