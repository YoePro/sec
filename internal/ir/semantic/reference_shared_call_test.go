package semantic

import (
	"os"
	"testing"
)

// P15-47: mutable callers may grant shared call authority without moving or
// copying the exclusive holder; returned shared references retain its origin.
// Rules: rules/memory/borrowing.md — §15.3(2), §15.4;
// rules/mlir/packages/sec-mlir-dialect_package15.md — §§59–63.
func TestPackage15SharedReferenceCallReborrow(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/reference_shared_call_reborrow.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		module, err := buildPackage15Source(t, string(source), triple)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Forward", "ReturnShared", "main"} {
			count := 0
			for _, block := range functionNamed(t, module, name).Blocks {
				for _, op := range block.Operations {
					if op.Kind == OpReferenceReborrowShared {
						count++
						if op.Reference == nil || op.Reference.SourceBorrow == 0 || op.Reference.Borrow == op.Reference.SourceBorrow || op.Results[0].Ownership != OwnershipSharedReference {
							t.Fatalf("%s lacks distinct shared call authority", name)
						}
					}
				}
			}
			want := 1
			if name == "Forward" {
				want = 2
			}
			if count != want {
				t.Fatalf("%s: shared calls=%d, want %d", name, count, want)
			}
		}
		for _, block := range functionNamed(t, module, "SelectExact").Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpReferenceReborrowShared {
					t.Fatal("shared reborrow displaced the exact mutable overload")
				}
			}
		}
	}
}
