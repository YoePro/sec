package semantic

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"sec/internal/sema"
)

// Generic storage must be closed even when a nominal instance is cached.
// Rules: rules/compiler/generics_lowering.md — §§4,7–8;
// rules/declarations/generics.md — §§23,25.
func TestGenericStorageClosureBeforeInterning(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/generic_storage_closure.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	box := a.Functions()["Read"][0].Parameters[0].Type
	choice := a.Functions()["Keep"][0].Parameters[0].Type
	invalid := sema.Type{Name: "Pending", Kind: sema.InvalidType}
	type storageCase struct {
		good, bad sema.Type
		path      string
	}
	cases := map[string]storageCase{}
	badBox := box
	badBox.Fields = append([]sema.StructField(nil), box.Fields...)
	badBox.Fields[0].Type = invalid
	cases["field"] = storageCase{box, badBox, "Value"}
	badChoice := choice
	badChoice.UnionVariants = append([]sema.UnionVariant(nil), choice.UnionVariants...)
	badChoice.UnionVariants[0].Payload = &invalid
	cases["payload"] = storageCase{choice, badChoice, "Item"}
	badNamed := choice
	badNamed.UnionVariants = append([]sema.UnionVariant(nil), choice.UnionVariants...)
	badNamed.UnionVariants[1].PayloadFields = append([]sema.StructField(nil), choice.UnionVariants[1].PayloadFields...)
	badNamed.UnionVariants[1].PayloadFields[0].Type = badBox
	cases["named-payload"] = storageCase{choice, badNamed, "Named.Value.Value"}
	for name, tc := range cases {
		for _, cached := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cached=%t", name, cached), func(t *testing.T) {
				m := &Module{Version: Version, Identity: "main", Types: NewTypeTable(), PointerWidthBits: 64}
				b := &builder{module: m, analyzer: a, maxPackage: 16, definedStructs: map[TypeID]bool{}, definedUnions: map[TypeID]bool{}, definedEnums: map[TypeID]bool{}}
				if cached {
					if _, err := b.internType(tc.good); err != nil {
						t.Fatal(err)
					}
				}
				before := Format(m)
				beforeTypes := len(m.Types.All())
				_, err := b.internType(tc.bad)
				var unsupported *UnsupportedFeatureError
				if !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "unresolved storage") || !strings.Contains(unsupported.Feature, tc.path) {
					t.Fatalf("incomplete %s accepted or lost path: %v", name, err)
				}
				if after := Format(m); after != before || len(m.Types.All()) != beforeTypes {
					t.Fatalf("rejected metadata allocated partial IR:\n%s", after)
				}
				if concreteSemaType(tc.bad) {
					t.Fatal("specialization closure accepted InvalidType storage")
				}
			})
		}
	}
}

// A bad retained source fact must fail the public builder, not publish a
// partially constructed module for Sec MLIR or ABI consumers.
// Rules: rules/compiler/generics_lowering.md — §§4,7–8.
func TestGenericStorageClosureBuildRejectsIncompleteFacts(t *testing.T) {
	for _, function := range []string{"Read", "Keep"} {
		t.Run(function, func(t *testing.T) {
			program, a := sliceBoundaryInput(t, "semantic_ir/generic_storage_closure.sec")
			typ := a.Functions()[function][0].Parameters[0].Type
			// White-box injection of incomplete metadata into the retained Sema fact.
			if function == "Read" {
				typ.Fields[0].Type = sema.Type{Name: "Pending", Kind: sema.InvalidType}
			} else {
				*typ.UnionVariants[0].Payload = sema.Type{Name: "Pending", Kind: sema.InvalidType}
			}
			module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
			if err == nil || !strings.Contains(err.Error(), "unresolved storage") || module != nil {
				t.Fatalf("incomplete facts crossed Build boundary: module=%v error=%v", module, err)
			}
		})
	}
}

// Closure covers nested storage carriers, arguments and callable signatures;
// unresolved facts behind indirection are not hidden by a shallow root check.
// Rules: rules/compiler/generics_lowering.md — §§4–5;
// rules/compiler/monomorphization.md — §§8.2,13.3.
func TestGenericStorageClosureNestedCarriers(t *testing.T) {
	for _, kind := range []sema.TypeKind{sema.InvalidType, sema.GenericType, sema.GenericTypePackType} {
		unresolved := sema.Type{Name: "Pending", Kind: kind}
		for name, typ := range map[string]sema.Type{
			"argument[0]":  {Kind: sema.ResultType, TypeArgs: []sema.Type{unresolved}},
			"element":      {Kind: sema.ReferenceType, Element: &unresolved},
			"base":         {Kind: sema.IntType, NamedBase: &unresolved},
			"return":       {Kind: sema.FunctionType, FunctionReturnType: &unresolved},
			"parameter[0]": {Kind: sema.FunctionType, FunctionParameterTypes: []sema.Type{unresolved}},
		} {
			if path := unresolvedConcreteStorage(typ, "Box.Value"); path != "Box.Value."+name {
				t.Errorf("kind=%v carrier=%s path=%q", kind, name, path)
			}
			if concreteSemaType(typ) {
				t.Errorf("kind=%v carrier=%s considered concrete", kind, name)
			}
		}
	}
}
