package semantic

import (
	"os"
	"testing"
)

// Reading whole self copies a copy-trivial value through the existing receiver
// Place, preserving shared/mutable parameter mode and later receiver uses.
// Rules: rules/declarations/functions.md §22; rules/memory/copy_move.md §18;
// rules/compiler/semantic_ir.md §§60(3),61(4); rules/mlir/packages/sec-mlir-dialect_package15.md §59.
func TestPackage15CopiesTrivialReceiverValues(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/receiver_value_copy.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Counter.Copy", "Counter.ResetAndCopy", "Number.Copy"} {
		fn := functionNamed(t, module, name)
		receiverType, _ := module.Types.Lookup(fn.Parameters[0].Value.Type)
		if receiverType.Kind != TypeReference || receiverType.Mutable != (name == "Counter.ResetAndCopy") {
			t.Fatal("wrong receiver capability", fn)
		}
		wholeReads := 0
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpPlaceRead {
					wholeReads++
				}
				if op.Kind == OpReferenceMove || op.Kind == OpReferenceEndBorrow {
					t.Fatal("copy ended receiver lifetime", op)
				}
			}
		}
		if wholeReads != 1 {
			t.Fatalf("%s has %d reads", name, wholeReads)
		}
	}
}
