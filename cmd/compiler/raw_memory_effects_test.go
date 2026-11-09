package main

import (
	"strings"
	"testing"
)

// Analysis presentation retains the shared registry's ordinary/volatile event
// identities and source sites. Merely holding an address adds no access.
// Rules: rules/memory/raw_pointers.md §31(1–6).
func TestRawMemoryEffectAnalysis(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/raw_memory_effects.sec")
	output := stdout + stderr
	if code != 0 {
		t.Fatalf("code %d: %s", code, output)
	}
	for _, want := range []string{"Ordinary: memory-write at", "Ordinary: memory-read at", "Volatile: volatile-write at", "Volatile: volatile-read at"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q: %s", want, output)
		}
	}
	for _, unwanted := range []string{"Ordinary: volatile-", "Possess: memory-", "ReadOnly: volatile-"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("invented %q: %s", unwanted, output)
		}
	}
}
