package main

import (
	"os"
	"testing"
)

// LSP shares the same ordinary Result/try boundaries as compiler checking.
// Rules: rules/errors/errorhandling.md — §§7,9,13,14,26;
// rules/declarations/enums.md — §12.1; rules/tooling/lsp.md — Diagnostics.
func TestEnumConversionOrdinaryTryLSP(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_conversion_try/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := 0
		for _, d := range analyze("file:///tmp/sec-enum-try/main.sec", string(data)) {
			if d.Severity == 1 {
				errors++
			}
		}
		if name == "valid" && errors != 0 || name == "invalid" && errors != 7 {
			t.Fatalf("%s: %d errors", name, errors)
		}
	}
}
