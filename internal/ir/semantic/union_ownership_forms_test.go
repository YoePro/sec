package semantic

import (
	"os"
	"testing"
)

// TestUnionOwnershipFormsIR preserves the ordinary borrowed factory's source
// owner and lowers equivalent named/unnamed constructor ownership plans.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/memory/destruction.md §§6,9; rules/compiler/semantic_ir.md §§23,35–36.
func TestUnionOwnershipFormsIR(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/sema/union_ownership_forms_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, fn := range module.Functions {
		if fn.Name != "Ordinary" {
			continue
		}
		found = true
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpOwnMoveFromPlace || op.Kind == OpOwnMove || op.Kind == OpCleanupCancel {
					t.Fatalf("borrowed factory call consumed its owner: %+v", op)
				}
			}
		}
	}
	if !found {
		t.Fatal("ordinary factory caller missing")
	}
}
