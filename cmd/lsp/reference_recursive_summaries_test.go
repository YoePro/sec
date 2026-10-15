package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LSP uses the same recursive lifetime proof and conservative failures as CLI.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3;
// rules/tooling/lsp.md — shared semantic diagnostics.
func TestRecursiveReferenceSummariesLSP(t *testing.T) {
	for _, test := range []struct {
		file, message string
		count         int
	}{
		{"reference_recursive_summaries.sec", "", 0},
		{"reference_recursive_unseeded_invalid.sec", "unknown control-flow provenance", 3},
		{"reference_recursive_local_invalid.sec", "cannot return reference to local", 2},
	} {
		source, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		diagnostics := analyze(uriFromPath(filepath.Join(t.TempDir(), test.file)), string(source))
		if test.message == "" {
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
		} else if len(diagnostics) != test.count || !strings.Contains(diagnostics[0].Message, test.message) {
			t.Fatal(test.file, diagnostics)
		}
	}
}
