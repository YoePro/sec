package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTryNegativeLiteralLSP consumes the shared failure proof without adding
// editor-local exceptions, retaining one diagnostic for dynamic negation.
// Rules: rules/errors/runtime_checks.md — §§5.1,7;
// rules/tooling/lsp.md — Diagnostics.
func TestTryNegativeLiteralLSP(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/try_negative_literal_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity == 1 {
				count++
				if d.Code != "S1093" {
					t.Fatal(d)
				}
			}
		}
		want := 0
		if mode == "invalid" {
			want = 1
		}
		if count != want {
			t.Fatal(mode, count)
		}
	}
}
