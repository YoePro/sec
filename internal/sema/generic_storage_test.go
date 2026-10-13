package sema

import (
	"os"
	"strings"
	"testing"
)

// TestGenericRecursiveStorageBoundaries exercises both template analysis and
// concrete substitution, preserving descriptors and rejecting inline storage.
// Rules: rules/declarations/unions.md — §18;
// rules/memory/layout.md — §13(7), §25(1–5).
func TestGenericRecursiveStorageBoundaries(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/recursive_boundaries_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	expected := map[string]TypeKind{
		"SharedValue": ReferenceType, "MutableValue": ReferenceType,
		"DynamicValue": ArrayType, "ViewValue": ReferenceType,
		"PointerValue": RawPtrType, "CollectionValue": StructType,
		"ChangingReferenceValue": ReferenceType, "MapValue": StructType, "SetValue": StructType,
	}
	for name, kind := range expected {
		function := a.Functions()[name][0]
		owner := function.Parameters[0].Type
		if len(owner.UnionVariants) != 2 {
			t.Fatal(name, owner)
		}
		variant := owner.UnionVariants[0]
		var payload Type
		if name == "MutableValue" {
			if len(variant.PayloadFields) != 1 {
				t.Fatal(name, variant)
			}
			payload = variant.PayloadFields[0].Type
		} else {
			if variant.Payload == nil {
				t.Fatal(name, variant)
			}
			payload = *variant.Payload
		}
		if payload.Kind != kind {
			t.Fatal(name, payload)
		}
		if name == "MutableValue" && !payload.ReferenceMutable {
			t.Fatal("mutable reference lost its capability", payload)
		}
		if name == "SharedValue" && (payload.Element == nil || len(payload.Element.TypeArgs) != 1 || payload.Element.TypeArgs[0].Name != "int") {
			t.Fatal("reference referent did not receive concrete arguments", payload)
		}
		if kind == ArrayType && arrayShapeOf(payload) != ArrayShapeDynamic {
			t.Fatal(name, payload)
		}
	}
	for _, name := range []string{"ReferenceArrayValue", "DynamicStructValue"} {
		owner := a.Functions()[name][0].Parameters[0].Type
		if len(owner.Fields) != 1 {
			t.Fatal(name, owner)
		}
		field := owner.Fields[0].Type
		if genericStructFieldHasDirectRecursiveStorage(owner, field) {
			t.Fatal("descriptor became inline storage", name)
		}
	}
	data, err = os.ReadFile("../../testdata/generics/recursive_boundaries_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors = analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 4 {
		t.Fatal(errors)
	}
	for _, diagnostic := range errors {
		if !strings.Contains(diagnostic.Message, "infinite size") || strings.Contains(diagnostic.Message, "does not converge") {
			t.Fatal(diagnostic)
		}
	}
}
