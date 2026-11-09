package main

import (
	"os"
	"strings"
	"testing"
)

// TestAsmOperandTypesLSP consumes the compiler's exact operand type decisions
// and registered diagnostics, without treating unsafe as a type waiver.
// Rules: rules/platform/inline_assembly.md — §§22,26–27;
// rules/tooling/lsp.md — diagnostics.
func TestAsmOperandTypesLSP(t *testing.T) {
	uri := "file:///asm-operand-types/main.sec"
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
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1207" || !strings.Contains(d.Message, "direct register operand") {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
