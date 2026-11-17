package main

import (
	"os"
	"strings"
	"testing"
)

// TestNumericConversionConstantFailuresLSP preserves the canonical compile-time
// conversion failures under try in editor diagnostics as well as CLI checks.
// Rules: rules/types/types.md — §25(9–11); rules/tooling/lsp.md — Diagnostics.
func TestNumericConversionConstantFailuresLSP(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "numeric_conversion_constants_valid.sec"
		if !valid {
			name = "numeric_conversion_constants_invalid.sec"
		}
		data, err := os.ReadFile("../../testdata/sema/" + name)
		if err != nil {
			t.Fatal(err)
		}
		diagnostics := analyze("file:///tmp/sec-numeric-conversion-constants/main.sec", string(data))
		errors := 0
		for _, d := range diagnostics {
			if d.Severity != 1 {
				continue
			}
			errors++
			if valid || !strings.Contains(d.Message, "constant conversion from") {
				t.Fatalf("unexpected diagnostic: %+v", d)
			}
		}
		want := 0
		if !valid {
			want = 12
		}
		if errors != want {
			t.Fatalf("got %d errors, want %d: %+v", errors, want, diagnostics)
		}
	}
}
