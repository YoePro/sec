package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// TestDeferFallibleHandlingLSP publishes the compiler's cleanup-local failure
// policy for unsaved source and clears errors after valid replacement.
// Rules: rules/control-flow/defer.md §15; rules/tooling/lsp.md — Shared diagnostic model.
func TestDeferFallibleHandlingLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"invalid", "valid"} {
		source, err := os.ReadFile("../../testdata/control_flow/defer_fallible/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		counts := map[string]int{}
		for _, d := range analyze(uri, string(source)) {
			if d.Severity != 1 {
				continue
			}
			if d.Range.Start.Line < 0 || d.Range.End.Character <= d.Range.Start.Character {
				t.Fatalf("missing source span: %+v", d)
			}
			counts[d.Code]++
		}
		if name == "valid" {
			if len(counts) != 0 {
				t.Fatal(counts)
			}
		} else if len(counts) != 3 || counts[diagnostics.UnhandledMustUseResult] != 1 || counts[diagnostics.TryPropagationIncompatible] != 4 || counts[diagnostics.TryResidualUnpropagatable] != 2 {
			t.Fatal(counts)
		}
	}
}
