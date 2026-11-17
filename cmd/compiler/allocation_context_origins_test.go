package main

import (
	"strings"
	"testing"
)

// Public analysis exposes creation/owner locations with the same shared
// descriptions as LSP, preserving the recorded semantic backing context
// separately from the created domain and unresolved physical provider.
// Rules: rules/memory/allocation.md §§5,6,29;
// rules/memory/arena.md §§4.2,4.3,67(4);
// rules/mlir/packages/sec-mlir-dialect_package19.md — §§52–54,72–76.
func TestAllocationContextOriginsCLI(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/allocation_context_facts_valid.sec")
	text := stdout + stderr
	if code != 0 {
		t.Fatalf("exit %d: %s", code, text)
	}
	for _, want := range []string{"allocation context origin:", "allocation_context_facts_valid.sec:11:", "allocation_context_facts_valid.sec:25:", "created domain origin:", "allocation_context_facts_valid.sec:30:", "backing allocation context: available; profile hosted; origin Arena", "domain identity unresolved"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing", want, text)
		}
	}
}
