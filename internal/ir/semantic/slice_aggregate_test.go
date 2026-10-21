package semantic

import (
	"fmt"
	"sec/internal/layout"
	"sec/internal/sema"
	"strings"
	"testing"
)

// Nested P13 fields supply P14 fixed-array Places, not copied arrays. Empty
// fields retain zero extent and borrowed fields retain their granting reference.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§114–115,140.
func TestPackage16AggregateArrayPlaces(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program := sliceBoundaryProgram(t, "semantic_ir/slice_aggregate_arrays.sec")
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
			if err != nil {
				t.Fatal(err)
			}
			if err = Verify(m); err != nil {
				t.Fatal(err)
			}
			for _, fn := range m.Functions {
				projections, borrows := 0, 0
				values := map[ValueID]TypeID{}
				for _, param := range fn.Parameters {
					values[param.Value.ID] = param.Value.Type
				}
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						for _, v := range op.Results {
							values[v.ID] = v.Type
						}
						switch op.Kind {
						case OpStructExtractField, OpStructSpreadFields, OpStructConstruct, OpArrayExtract, OpArrayReplace, OpStorageLoad:
							t.Fatalf("%s copied backing aggregate through %s", fn.Name, op.Kind)
						case OpPlaceField:
							projections++
						case OpSliceBorrowShared, OpSliceBorrowMutable:
							borrows++
							source, _ := m.Types.Lookup(values[op.Operands[0]])
							array, _ := m.Types.Lookup(source.Element)
							want := "4"
							if fn.Name == "Empty" {
								want = "0"
							}
							if source.Kind != TypePlace || array.Kind != TypeArray || array.Length != want || op.Slice == nil || len(op.Slice.OriginPlaces) != 1 || op.Slice.OriginPlaces[0] != op.Operands[0] {
								t.Fatalf("%s lost exact array Place/origin: %+v", fn.Name, op)
							}
							if fn.Name == "NestedMutable" && (op.Reference.SourceBorrow != fn.Parameters[0].Reference.Borrow || !source.Mutable) {
								t.Fatal("nested mutable projection lost granting borrow/authority")
							}
						}
					}
				}
				want := 2
				if fn.Name == "Empty" {
					want = 1
				}
				if projections != want || borrows != 1 {
					t.Fatalf("%s projections=%d borrows=%d", fn.Name, projections, borrows)
				}
			}
		})
	}
}

// Hand-built IR must not bypass Sema's move-only classification with a claimed
// trivial struct or a copy-trivial field action. Shared holders remain ordinary
// abstract fields; projected holder origin production stays gated by P16-36.
// Rules: rules/memory/copy_move.md — §§4.1–4.4;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39,114.
func TestPackage16AggregateSliceCopyBoundary(t *testing.T) {
	for _, shape := range []string{"direct", "named", "array", "nested", "result", "shared-referent"} {
		for _, mutable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", shape, mutable), func(t *testing.T) {
				types := NewTypeTable()
				element := types.Intern(Type{Kind: TypeInt, Name: "int32", Signed: true, BitWidth: 32})
				slice, err := types.InternSlice(element, mutable)
				if err != nil {
					t.Fatal(err)
				}
				m := &Module{Version: Version, Identity: "main", Types: types}
				field := slice
				switch shape {
				case "named":
					field = types.Intern(Type{Kind: TypeNamed, Name: "View", Identity: "main::View", Base: slice})
				case "array":
					field = types.Intern(Type{Kind: TypeArray, Element: slice, Length: "2"})
				case "result":
					field = types.Intern(Type{Kind: TypeResult, Success: slice, Error: element})
				case "shared-referent":
					field, err = types.InternSlice(slice, false)
					if err != nil {
						t.Fatal(err)
					}
				}
				addStruct := func(name string, field TypeID, classification string) TypeID {
					id := types.Intern(Type{Kind: TypeStruct, Name: name, Identity: "main::" + name})
					m.Structs = append(m.Structs, StructDefinition{TypeID: id, SymbolID: SymbolID("main::" + name), Name: name, CopyClassification: classification, TriviallyDestructible: true, Fields: []StructFieldDefinition{{ID: 0, Name: "Data", Type: field}}})
					return id
				}
				exclusive := mutable && shape != "shared-referent"
				classification := "trivial"
				if exclusive {
					classification = "move-only"
				}
				if shape == "nested" {
					field = addStruct("Inner", field, classification)
				}
				structure := addStruct("Views", field, classification)
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				if exclusive {
					m.Structs[len(m.Structs)-1].CopyClassification = "trivial"
					if err := Verify(m); err == nil || !strings.Contains(err.Error(), "mutable slice must be move-only") {
						t.Fatal("forged classification accepted", err)
					}
					m.Structs[len(m.Structs)-1].CopyClassification = classification
				}
				value := func(id ValueID, typ TypeID) Value { return Value{ID: id, Type: typ, Ownership: OwnershipImmediate} }
				for _, kind := range []OpKind{OpStructConstruct, OpStructSpreadFields, OpStructExtractField, OpStructReplaceField} {
					fn := &Function{ID: "main::Probe", Name: "Probe", ReturnType: field, Entry: 0, Parameters: []Parameter{{Name: "view", Value: value(1, structure)}, {Name: "field", Value: value(2, field)}}}
					op := Operation{Kind: kind, Results: []Value{value(3, field)}, StructActions: []StructFieldAction{StructActionCopyTrivial}, StructOrigins: []StructFieldOrigin{StructOriginExplicit}}
					switch kind {
					case OpStructConstruct:
						op.Operands = []ValueID{2}
						op.Results[0].Type = structure
					case OpStructSpreadFields, OpStructExtractField:
						op.Operands = []ValueID{1}
					case OpStructReplaceField:
						op.Operands = []ValueID{1, 2}
						op.Results[0].Type = structure
					}
					fieldType, _ := types.Lookup(field)
					if fieldType.Kind == TypeSharedSlice || fieldType.Kind == TypeMutableSlice {
						fn.Parameters[1].Reference = p15Proven(2, 0)
					}
					fn.ReturnType = types.Intern(Type{Kind: TypeVoid})
					fn.Blocks = []*Block{{ID: 0, Operations: []Operation{op, {Kind: OpReturn}}}}
					m.Functions = []*Function{fn}
					err := Verify(m)
					if exclusive {
						if err == nil || !strings.Contains(err.Error(), "mutable slice") {
							t.Fatalf("%s accepted exclusive field copy: %v", kind, err)
						}
					} else if err != nil {
						t.Fatalf("%s rejected shared field: %v", kind, err)
					}
				}
			})
		}
	}
}
