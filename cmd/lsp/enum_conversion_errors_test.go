package main

import (
	"os"
	"strings"
	"testing"
)

// LSP shares the canonical enum narrowing channel and preserves the known-value
// compile-time rejection instead of accepting the legacy arithmetic channel.
// Rules: rules/types/types.md — "Explicit conversions" (MD-012, MD-104);
// rules/declarations/enums.md — §12.1; rules/tooling/lsp.md — Diagnostics.
func TestEnumConversionErrorChannelLSP(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_conversion_errors/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-enum-conversion-errors/main.sec", string(data))
		errors, canonical, constant := 0, false, false
		for _, d := range values {
			if d.Severity != 1 {
				continue
			}
			errors++
			canonical = canonical || strings.Contains(d.Message, "Result[uint8, ConversionError]")
			constant = constant || strings.Contains(d.Message, "300 overflows uint8")
		}
		if name == "valid" && errors != 0 || name == "invalid" && (errors != 4 || !canonical || !constant) {
			t.Fatalf("%s: %+v", name, values)
		}
	}
}
