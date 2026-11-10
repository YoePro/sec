package main

import (
	"os"
	"strings"
	"testing"
)

// TestAsmVolatileSyntaxLSP projects the canonical parser diagnostic without dependent Sema errors.
// Rules: rules/platform/inline_assembly.md — §46; rules/tooling/lsp.md — diagnostics.
func TestAsmVolatileSyntaxLSP(t *testing.T) {
	uri := "file:///asm-volatile/main.sec"
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"asm_volatile_identifier_valid.sec", 0}, {"asm_volatile_invalid.sec", 3},
	} {
		data, err := os.ReadFile("../../testdata/parser/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "P2012" || !strings.Contains(d.Message, "use ordinary asm") {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
