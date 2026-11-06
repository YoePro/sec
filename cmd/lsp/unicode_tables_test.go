package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Generated rune extensions and unicode module wrappers are visible through
// the same core/import analysis used for editor diagnostics.
// Rules: rules/library/core-library.md — §11.3; rules/tooling/lsp.md — Shared diagnostic model.
func TestGeneratedUnicodePropertiesLSP(t *testing.T) {
	path, err := filepath.Abs("../../testdata/unicode_classification_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range analyze(uriFromPath(path), string(source)) {
		if d.Severity == 1 {
			t.Fatalf("Unicode property rejected: %+v", d)
		}
	}
}
