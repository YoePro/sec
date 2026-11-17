package main

import (
	"strings"
	"testing"
)

// sec analyse reports the same non-panic effect cause chains as LSP hover.
// Rules: rules/compiler/semantic_ir.md — § 109(1)–(3).
func TestAnalyseEffectCauseChains(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/tooling/effect_cause_chains.sec")
	output := stdout + stderr
	if code != 0 {
		t.Fatalf("code %d: %s", code, output)
	}
	for _, want := range []string{"Leaf: memory-write at", "Wrapper: memory-write via Wrapper -> Leaf (introduced at", "Outer: memory-write via Outer -> Wrapper -> Leaf (introduced at"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Plain: memory-") {
		t.Errorf("Plain was given an effect:\n%s", output)
	}
}
