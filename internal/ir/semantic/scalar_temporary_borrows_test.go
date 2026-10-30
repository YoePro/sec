package semantic

import (
	"os"
	"testing"
)

// Literal/operator rvalues need the same stable call-lifetime temporary Places
// as freshly constructed values; shared borrows do not copy an existing Place,
// and mutable scalar temporaries use address-taken storage.
// Rules: rules/memory/borrowing.md §5.4(1–3),(6–7); rules/compiler/semantic_ir.md §45.
func TestScalarTemporaryCallBorrows(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/scalar_temporary_borrows.sec")
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
	use := functionNamed(t, module, "Use")
	shared, mutable := 0, 0
	for _, block := range use.Blocks {
		for _, op := range block.Operations {
			switch op.Kind {
			case OpReferenceBorrowShared:
				shared++
			case OpReferenceBorrowMutable:
				mutable++
			}
		}
	}
	if shared != 2 || mutable != 1 {
		t.Fatalf("missing scalar temporary borrows: shared %d mutable %d", shared, mutable)
	}
	if len(use.Storages) != 1 || !use.Storages[0].AddressTaken {
		t.Fatal("mutable literal has no address-taken owner")
	}
}
