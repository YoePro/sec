package semantic

import (
	"sec/internal/sema"
	"testing"
)

// The backend key must preserve source input identity even when presentation
// names or physical carriers coincide. Outer result/consuming mode, names,
// receiver authority and safety/effect annotations do not authorize overloads.
// Rules: rules/declarations/functions.md §§11,19–20;
// rules/types/types.md — Type identity, Arrays, References, Function types;
// rules/compiler/semantic_ir.md §61.
func TestOverloadCompleteSignatureIdentity(t *testing.T) {
	integer := sema.Type{Name: "int", Kind: sema.IntType}
	boolean := sema.Type{Name: "bool", Kind: sema.BoolType}
	left := sema.Type{Name: "Value", Module: "left", Kind: sema.IntType, Named: true, Declared: true}
	right := left
	right.Module = "right"
	key := func(types ...sema.Type) FunctionID {
		f := sema.Function{Module: "main", Name: "Select"}
		for _, typ := range types {
			f.Parameters = append(f.Parameters, sema.FunctionParameter{Type: typ})
		}
		return functionID(f, nil)
	}
	callback := func(parameter, result sema.Type, capability sema.CallableCapability, variadic bool) sema.Type {
		return sema.Type{Name: "same display", Kind: sema.FunctionType, FunctionParameterTypes: []sema.Type{parameter}, FunctionReturnType: &result, FunctionCapability: capability, FunctionVariadic: variadic}
	}
	for _, tc := range []struct {
		name string
		a, b sema.Type
	}{
		{"callback parameter module", callback(left, integer, sema.CallableShared, false), callback(right, integer, sema.CallableShared, false)},
		{"callback result module", callback(integer, left, sema.CallableShared, false), callback(integer, right, sema.CallableShared, false)},
		{"callback capability", callback(integer, integer, sema.CallableShared, false), callback(integer, integer, sema.CallableMutable, false)},
		{"consuming callback", callback(integer, integer, sema.CallableMutable, false), callback(integer, integer, sema.CallableConsuming, false)},
		{"callback variadic shape", callback(integer, integer, sema.CallableShared, false), callback(integer, integer, sema.CallableShared, true)},
		{"owning array versus slice", sema.Type{Name: "int[]", Kind: sema.ArrayType, ArrayShape: sema.ArrayShapeDynamic, Element: &integer}, sema.Type{Name: "int[]", Kind: sema.SliceType, Element: &integer}},
		{"slice authority", sema.Type{Name: "int[]", Kind: sema.SliceType, Element: &integer}, sema.Type{Name: "int[]", Kind: sema.SliceType, Element: &integer, ReferenceMutable: true}},
		{"reference authority", sema.Type{Name: "same", Kind: sema.ReferenceType, Element: &left}, sema.Type{Name: "same", Kind: sema.ReferenceType, Element: &left, ReferenceMutable: true}},
		{"fixed lengths", sema.Type{Name: "same", Kind: sema.ArrayType, ArrayShape: sema.ArrayShapeFixed, ArrayLengthDecimal: "18446744073709551616", Element: &left}, sema.Type{Name: "same", Kind: sema.ArrayType, ArrayShape: sema.ArrayShapeFixed, ArrayLengthDecimal: "18446744073709551617", Element: &left}},
		{"ordered generic arguments", sema.Type{Name: "Pair", Module: "m", Named: true, TypeArgs: []sema.Type{integer, boolean}}, sema.Type{Name: "Pair", Module: "m", Named: true, TypeArgs: []sema.Type{boolean, integer}}},
		{"constant arguments", sema.Type{Name: "Matrix", Module: "m", Named: true, ConstArgs: []int64{2, 3}}, sema.Type{Name: "Matrix", Module: "m", Named: true, ConstArgs: []int64{3, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if key(tc.a) == key(tc.b) {
				t.Fatalf("signature collision: %s", key(tc.a))
			}
		})
	}
	if key(integer, boolean) == key(boolean, integer) || key(integer) == key(integer, integer) {
		t.Fatal("ordered parameters/arity collapsed")
	}
	shared := callback(left, integer, sema.CallableShared, false)
	implicit := shared
	implicit.FunctionCapability = ""
	implicit.Name = "another presentation"
	if key(shared) != key(implicit) {
		t.Fatal("implicit shared capability or presentation changed identity")
	}
	base := sema.Function{Module: "m", Name: "Select", Parameters: []sema.FunctionParameter{{Name: "before", Type: integer}}, ReturnType: integer}
	changed := base
	changed.Parameters = append([]sema.FunctionParameter(nil), base.Parameters...)
	changed.Parameters[0].Name = "after"
	changed.Parameters[0].Consuming = true
	changed.ReturnType = boolean
	changed.Unsafe = true
	changed.Static = true
	changed.ReceiverMutable = true
	if functionID(base, nil) != functionID(changed, nil) {
		t.Fatal("non-discriminating dimension changed identity")
	}
	changed = base
	changed.Parameters = append([]sema.FunctionParameter(nil), base.Parameters...)
	changed.Parameters[0].Variadic = true
	if functionID(base, nil) == functionID(changed, nil) {
		t.Fatal("fixed and native variadic input shape collapsed")
	}
}
