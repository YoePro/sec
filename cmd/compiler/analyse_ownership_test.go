package main

import (
	"strings"
	"testing"
)

// sec analyse reports every accepted move with its source Place, owning
// destination and post-move source state, and reads under partial aggregate
// state with the moved parts and where they moved, from the same Sema facts
// the LSP hovers consume; copies are not listed.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 109(1)–(3)
//   - rules/memory/copy_move.md — §§ 8–11, 29(1)–(2)
func TestAnalyseOwnershipTransitions(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/tooling/ownership_transitions.sec")
	output := stdout + stderr
	if code != 0 {
		t.Fatalf("code %d: %s", code, output)
	}
	if strings.Count(output, "ownership:") != 1 {
		t.Errorf("want exactly one ownership section:\n%s", output)
	}
	for _, want := range []string{
		"Split: partial move of pair.left to argument 1 of Consume at",
		"Split: partial move of pair.right to initialization right at",
		"Split: move of right to argument 1 of Consume at",
		"Keep: move of token to initialization moved at",
		"Keep: move of moved to return at",
		"read of pair.right at",
		"with unavailable parts: pair.left moved at",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "copy of") {
		t.Errorf("copies were listed:\n%s", output)
	}
}
