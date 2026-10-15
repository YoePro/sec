package main

import (
	"os"
	"strings"
	"testing"
)

// rules/memory/unsafe.md §7(1–5): LSP must apply the same explicit-body
// rule as the CLI, without losing callable requirements at call sites.
func TestUnsafeFunctionBodyDiagnosticsLSP(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"raw_pointer_unsafe_body_invalid.sec", 1},
		{"unsafe_function_asm_invalid.sec", 1},
		{"unsafe_callable_body_invalid.sec", 4},
		{"unsafe_callable_body_valid.sec", 0},
	} {
		t.Run(test.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/tooling/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			diagnostics := analyze("file:///tmp/sec-unsafe-body/"+test.file, string(source))
			if len(diagnostics) != test.errors {
				t.Fatalf("diagnostics = %+v, want %d", diagnostics, test.errors)
			}
			for _, diagnostic := range diagnostics {
				if !strings.Contains(diagnostic.Message, "requires unsafe") && !strings.Contains(diagnostic.Message, "only allowed inside unsafe") {
					t.Fatalf("unexpected diagnostic: %+v", diagnostic)
				}
			}
		})
	}
}
