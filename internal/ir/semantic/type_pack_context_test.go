package semantic

import (
	"os"
	"testing"
)

// Literal contexts and expression types from Sema must reach the existing pack
// specialization producer in direct-return, typed-let and inferred-let positions.
// Rules: rules/compiler/semantic_ir.md §27A; rules/declarations/functions.md §38A(4).
func TestTypePackFixedContextReachesIR(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/generics/type_pack_declarations/shared_fixed_context.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, function := range module.Functions {
		if function.TypePack == nil {
			continue
		}
		count++
		if function.TypePack.FixedCount != 1 || len(function.Parameters) != 1+len(function.TypePack.Types) {
			t.Fatalf("lost concrete receiving signature: %+v", function)
		}
	}
	if count != 3 {
		t.Fatalf("want three concrete pack arities, got %d", count)
	}
}
