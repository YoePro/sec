package semantic

import (
	"os"
	"testing"
)

// CollectionError retains one nominal enum identity and the complete public
// case inventory through Semantic IR, with a target-sized uint carrier.
// Rules: rules/collections/collections.md — §16;
// rules/compiler/semantic_ir.md — resolved types and enum case identity.
func TestCollectionErrorPreservesCanonicalEnumIdentity(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/collection_errors/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if len(module.Enums) != 1 || module.Enums[0].Name != "CollectionError" || len(module.Enums[0].Cases) != 3 {
		t.Fatalf("canonical enum definition lost: %+v", module.Enums)
	}
	carrier, ok := module.Types.Lookup(module.Enums[0].Underlying)
	if !ok || carrier.Kind != TypeUint || !carrier.TargetSize {
		t.Fatalf("CollectionError carrier is not target-sized uint: %+v", carrier)
	}
	typ, ok := module.Types.Lookup(module.Enums[0].TypeID)
	if !ok || typ.Kind != TypeEnum || typ.Identity != "core::CollectionError" {
		t.Fatalf("public error identity changed: %+v", typ)
	}
	for index, name := range []string{"AllocationFailed", "CapacityExceeded", "SizeOverflow"} {
		item := module.Enums[0].Cases[index]
		if item.Name != name || item.ID != EnumCaseID(index) || item.Value.Int64() != int64(index) {
			t.Fatalf("case identity changed: %+v", item)
		}
	}
	for index, name := range []string{"Allocation", "Capacity", "Overflow"} {
		fn := loopFunction(t, module, name)
		if countOperations(fn, OpEnumConstant) != 1 {
			t.Fatalf("%s did not build one canonical case", name)
		}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpEnumConstant && (op.EnumCase != EnumCaseID(index) || len(op.Results) != 1 || op.Results[0].Type != module.Enums[0].TypeID) {
					t.Fatalf("%s returned a different enum case: %+v", name, op)
				}
			}
		}
	}
}
