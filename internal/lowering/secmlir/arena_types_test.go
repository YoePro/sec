package secmlir

import (
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
)

// P19-14 adds semantic identities only. Emission must not invent schema-15
// Arena/context types or a physical ABI while that milestone remains open.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§84–85,105–109,146–148.
func TestArenaTypesAwaitSchema15(t *testing.T) {
	types := semantic.NewTypeTable()
	for _, id := range []semantic.TypeID{types.InternArena(), types.InternAllocationContext()} {
		e := &emitter{module: &semantic.Module{Types: types}, types: map[semantic.TypeID]string{}, visiting: map[semantic.TypeID]bool{}}
		text, err := e.typeText(id)
		if text != "" || err == nil || !strings.Contains(err.Error(), "semantic type") {
			t.Fatalf("unexpected schema support: %q %v", text, err)
		}
	}
}

// The high-level hidden context representation must not silently become an
// ordinary argument or disappear when schema 15 is unavailable.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§98,107–109.
func TestHiddenAllocationContextAwaitsSchema15(t *testing.T) {
	types := semantic.NewTypeTable()
	context := types.InternAllocationContext()
	void := types.Intern(semantic.Type{Kind: semantic.TypeVoid})
	module := &semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{{ID: "main::F", Name: "F", RequiresAllocationContext: true, ReturnType: void, Parameters: []semantic.Parameter{{Name: "$context", HiddenAllocationContext: true, Value: semantic.Value{ID: 0, Type: context, Ownership: semantic.OwnershipCompilerTemporary}}}, Blocks: []*semantic.Block{{ID: 0, Operations: []semantic.Operation{{Kind: semantic.OpReturn}}}}}}}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	output, err := Emit(module, layout.ResolvedScalarPlan{})
	if err == nil || len(output) != 0 {
		t.Fatalf("hidden context emitted before schema 15: %q %v", output, err)
	}
}
