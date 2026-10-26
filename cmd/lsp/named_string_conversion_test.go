package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// The LSP projects the same registered relation error and repair as the CLI;
// it never advertises an infallible numeric-to-nominal-string conversion.
// Rules: rules/types/types.md — "Numeric-to-text conversion";
// rules/tooling/diagnostics.md — § 15.
func TestNamedStringConversionLSP(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/named_string_numeric_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	values := analyze(uriFromPath(filepath.Join(t.TempDir(), "main.sec")), string(input))
	if len(values) != 12 {
		t.Fatalf("diagnostics: %+v", values)
	}
	for _, value := range values {
		if value.Code != diagnostics.ConversionUnavailable || value.Severity != 1 ||
			!strings.Contains(value.Message, "try Name(number.ToString())") {
			t.Fatalf("lost conversion identity/help: %+v", value)
		}
	}
}
