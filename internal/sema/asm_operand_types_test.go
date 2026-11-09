package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestAsmOperandTypes rejects high-level register operands on both input and
// output boundaries; explicit primitive extraction and RawPtr remain valid.
// Rules: rules/platform/inline_assembly.md — §§22,26–27.
func TestAsmOperandTypes(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"asm_operand_types_valid.sec", 0}, {"asm_operand_types_invalid.sec", 20},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		for _, err := range errors {
			if err.ID != diagnostics.AsmInvalidRegisterOperandType || !strings.Contains(err.Help, "Explicitly extract or convert") {
				t.Fatal(err)
			}
		}
	}
}
