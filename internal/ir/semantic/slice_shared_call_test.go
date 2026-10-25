package semantic

import (
	"os"
	"testing"
)

// Mutable slice arguments grant fresh shared authority rather than copying
// the mutable holder. The call is followed by a write through the surviving
// parent, and exact mutable overloads retain precedence over a shared reborrow.
// Rules: rules/memory/borrowing.md — §15.3(2), §15.4;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,64–65.
func TestPackage16SharedSliceCallReborrow(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_shared_call_reborrow.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Forward", "main"} {
			fn := functionNamed(t, module, name)
			shared := 0
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind == OpSliceMoveMutable || op.Kind == OpSliceCopyShared {
						t.Fatalf("%s transferred the mutable caller holder instead of reborrowing", name)
					}
					if op.Kind != OpSliceReborrowShared {
						continue
					}
					shared++
					if op.Reference == nil || op.Reference.SourceBorrow == 0 || op.Reference.Borrow == op.Reference.SourceBorrow || op.Results[0].Ownership != OwnershipSharedReference {
						t.Fatalf("%s lacks a distinct shared borrow: %+v", name, op)
					}
				}
			}
			want := 2
			if name == "main" {
				want = 1
			}
			if shared != want {
				t.Fatalf("%s/%d: shared reborrows=%d, want %d", name, width, shared, want)
			}
		}
		fn := functionNamed(t, module, "SelectExact")
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpSliceReborrowShared {
					t.Fatal("shared conversion displaced the exact mutable overload")
				}
			}
		}
	}
}
