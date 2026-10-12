package semantic

import (
	"os"
	"testing"
)

// TestProvenAssertionPreservesConditionEffects retains checked multiplication
// while eliminating only the proven assertion failure. Dropping the whole
// condition would erase its independent defined arithmetic failure.
// Rules: rules/errors/panic.md — §15.8; rules/compiler/semantic_ir.md — §56 "Assertion".
func TestProvenAssertionPreservesConditionEffects(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/assertion_condition_effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	function := loopFunction(t, module, "Product")
	if countOperations(function, OpIntBinaryChecked) != 1 {
		t.Fatal("proven assertion erased checked multiplication", Format(module))
	}
	for _, block := range function.Blocks {
		for _, op := range block.Operations {
			if op.AssertCheck || op.Kind == OpPanic {
				t.Fatal("proven assertion retained its failure check", op)
			}
		}
	}
}
