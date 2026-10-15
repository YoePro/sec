package semantic

import (
	"os"
	"strings"
	"testing"
)

// rules/memory/raw_pointers.md §§ 4, 23, 32: copying/passing a raw address
// preserves its category without creating a safe borrow or pointee owner.
func TestRawPointerValuesFromSource(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/raw_pointer_values_valid.sec")
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
	text := Format(module)
	for _, want := range []string{"rawptr<!", "[raw-pointer]", "ref<!", "main::Address", "storage.load", "call", "struct.construct", "array.construct"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	for _, fn := range module.Functions {
		for _, p := range fn.Parameters {
			if isRawPointerType(module.Types, p.Value.Type) && (p.Reference != nil || p.Value.Ownership != OwnershipRawPointer) {
				t.Errorf("raw parameter acquired safe facts: %+v", p)
			}
		}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				for _, value := range op.Results {
					if isRawPointerType(module.Types, value.Type) && (value.Ownership != OwnershipRawPointer || op.Reference != nil) {
						t.Errorf("raw result acquired safe facts: %+v", op)
					}
				}
			}
		}
	}
}

func TestRawPointerVerifierRejectsSafeClassification(t *testing.T) {
	for _, own := range []OwnershipClass{OwnershipOwned, OwnershipSharedReference, OwnershipMutableReference, OwnershipImmediate} {
		t.Run(string(own), func(t *testing.T) {
			types := NewTypeTable()
			void := types.Intern(Type{Kind: TypeVoid})
			raw := types.Intern(Type{Kind: TypeRawPointer, Element: void})
			if err := addValue(types, map[ValueID]Value{}, Value{ID: 1, Type: raw, Ownership: own}); err == nil {
				t.Fatal("accepted raw pointer with non-raw ownership")
			}
		})
	}
	types := NewTypeTable()
	void := types.Intern(Type{Kind: TypeVoid})
	raw := types.Intern(Type{Kind: TypeRawPointer, Element: void})
	for _, extern := range []bool{false, true} {
		fn := &Function{ID: "f", Name: "f", ReturnType: void, Extern: extern, Entry: 1,
			Parameters: []Parameter{{Name: "p", Value: Value{ID: 1, Type: raw, Ownership: OwnershipRawPointer}, Reference: &ReferenceMetadata{}}}}
		if !extern {
			fn.Blocks = []*Block{{ID: 1, Operations: []Operation{{Kind: OpReturn}}}}
		}
		module := &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "safe-reference facts") {
			t.Fatalf("error = %v", err)
		}
	}
	if err := addValue(types, map[ValueID]Value{}, Value{ID: 1, Type: void, Ownership: OwnershipRawPointer}); err == nil {
		t.Fatal("accepted non-pointer value with raw-pointer classification")
	}
	op := Operation{Results: []Value{{ID: 1, Type: raw, Ownership: OwnershipRawPointer}}, Reference: &ReferenceMetadata{}}
	if err := verifyOperation(&Module{Types: types}, nil, op, nil, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "safe-reference facts") {
		t.Fatalf("raw result verification = %v", err)
	}
}

func TestRawPointerTypeIdentityAndPointeeValidation(t *testing.T) {
	types := NewTypeTable()
	void := types.Intern(Type{Kind: TypeVoid})
	byteType := types.Intern(Type{Kind: TypeByte})
	raw := types.Intern(Type{Kind: TypeRawPointer, Element: byteType})
	ref := types.Intern(Type{Kind: TypeReference, Element: byteType})
	untyped := types.Intern(Type{Kind: TypeRawPointer, Element: void})
	if raw == ref || raw == untyped || raw != types.Intern(Type{Kind: TypeRawPointer, Element: byteType}) {
		t.Fatal("pointer categories or pointees collapsed")
	}
	if err := verifyTypes(types); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []Type{
		{Kind: TypeRawPointer},
		{Kind: TypeRawPointer, Element: byteType, Mutable: true},
		{Kind: TypeRawPointer, Element: types.Intern(Type{Kind: TypePlace, Element: byteType})},
		{Kind: TypeRawPointer, Element: types.Intern(Type{Kind: TypeNever})},
	} {
		if err := verifyRawPointerType(types, 99, typ); err == nil {
			t.Errorf("accepted invalid pointer: %+v", typ)
		}
	}
}
