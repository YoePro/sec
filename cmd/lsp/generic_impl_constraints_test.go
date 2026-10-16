package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenericImplConstraintsLSP checks the shared Sema decisions on unsaved
// source: inherited guarantees are valid; missing conjuncts stay diagnostics.
// Rules: rules/declarations/generics.md — §§8,12,14–15,25;
// rules/tooling/lsp.md — "Diagnostics".
func TestGenericImplConstraintsLSP(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"generic_impl_constraints_valid.sec", 0},
		{"generic_impl_constraints_invalid.sec", 8},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "constraints.sec"))
		count := 0
		for _, diagnostic := range analyze(uri, string(data)) {
			if diagnostic.Severity != 1 {
				continue
			}
			count++
			if !strings.Contains(diagnostic.Message, "does not satisfy constraint") {
				t.Fatal(diagnostic)
			}
		}
		if count != test.errors {
			t.Fatal(test.file, count)
		}
	}
}
