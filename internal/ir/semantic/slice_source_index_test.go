package semantic

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/sema"
)

// Source indexes form guarded P15 element Places for trivial reads/writes
// and shared/mutable borrows; try routes canonical IndexError without panic.
// Rules: rules/compiler/semantic_ir.md — §126.15–20;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45–58,132–133.
func TestPackage16SliceSourceIndex(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, _ := sliceBoundaryInput(t, "semantic_ir/slice_source_index.sec")
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(m); err != nil {
				t.Fatalf("%v\n%s", err, Format(m))
			}
			for _, fn := range m.Functions {
				predicates, places, reads, writes, panics, errors := 0, 0, 0, 0, 0, 0
				var ordered []OpKind
				for _, block := range fn.Blocks {
					for i, op := range block.Operations {
						ordered = append(ordered, op.Kind)
						switch op.Kind {
						case OpSliceIndexInBounds:
							predicates++
							branch := block.Operations[i+1]
							place := fn.Blocks[branch.Successors[0].Block].Operations[0]
							if place.Kind != OpSliceElementPlace || place.Operands[0] != op.Operands[0] || place.Operands[1] != op.Operands[1] || place.ArrayGuard != op.Results[0].ID {
								t.Fatal("predicate and element Place did not reuse the exact source/index SSA")
							}
							if fn.Name == "Read" || fn.Name == "Mutable" {
								indexType := "int128"
								if fn.Name == "Mutable" {
									indexType = "uint256"
								}
								found := false
								for _, parameter := range fn.Parameters {
									if parameter.Value.ID == op.Operands[1] {
										typ, _ := m.Types.Lookup(parameter.Value.Type)
										found = typ.Name == indexType
									}
								}
								if !found {
									t.Fatal("index was narrowed before bounds checking")
								}
							}
						case OpSliceElementPlace:
							places++
						case OpPlaceRead:
							reads++
						case OpPlaceWrite:
							writes++
						case OpBoundsFailure:
							if op.ArrayOperation != "slice-index" {
								t.Fatal("wrong slice bounds provenance")
							}
							panics++
						case OpEnumConstant:
							definition, ok := enumDefinition(m, op.Results[0].Type)
							if ok && definition.SymbolID == "core::IndexError" {
								if definition.Cases[op.EnumCase].Name != "OutOfBounds" {
									t.Fatal("wrong fallible index error")
								}
								errors++
							}
						case OpArrayExtract, OpArrayReplace, OpStorageLoad, OpSliceCopyShared, OpSliceMoveMutable:
							t.Fatal("indexing copied an aggregate, transferred a slice or loaded backing storage", op.Kind)
						}
					}
				}
				if fn.Name == "Proven" && (predicates != 0 || places != 1 || reads != 1 || panics != 0) {
					t.Fatal("proven read acquired a redundant bounds check")
				}
				if fn.Name == "Handled" || fn.Name == "HandledMutable" || fn.Name == "Propagate" || fn.Name == "Specific" || fn.Name == "Empty" {
					if predicates != 1 || places != 1 || panics != 0 || errors != 1 {
						t.Fatalf("%s lost typed fallible index CFG", fn.Name)
					}
				}
				if fn.Name == "Ordered" {
					var access []OpKind
					for _, kind := range ordered {
						if kind == OpDirectCall || kind == OpSliceElementPlace || kind == OpPlaceWrite {
							access = append(access, kind)
						}
					}
					want := []OpKind{OpDirectCall, OpSliceElementPlace, OpDirectCall, OpPlaceWrite}
					if fmt.Sprint(access) != fmt.Sprint(want) || writes != 1 || predicates != 1 {
						t.Fatal("destination Place was not established once before the RHS", access)
					}
				}
			}
		})
	}
}

// Reference-valued elements cannot bypass P15's unsupported value carrier
// boundary just because their language copy classification is trivial.
// The failed producer must return no module, including preceding functions.
// Rules: rules/compiler/semantic_ir.md — §126.19–20,25;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§55–58,128,137.
func TestPackage16SliceIndexElementValueBoundary(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/slice_index_reference_element.sec")
	m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
	var unsupported *UnsupportedFeatureError
	if m != nil || !errors.As(err, &unsupported) || !strings.Contains(err.Error(), "P15 trivial value surface") {
		t.Fatalf("unsupported element value leaked IR or lost its named boundary: %v", err)
	}
}

// Parent-holder end invalidates an element reference derived from its Place,
// and malformed spatial guard edges cannot authorize element access.
// Rules: rules/compiler/semantic_ir.md — §126.25;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§54,57,63,81–84.
func TestPackage16SliceSourceIndexInvalidEdges(t *testing.T) {
	for _, parentEnd := range []bool{false, true} {
		program, a := sliceBoundaryInput(t, "semantic_ir/slice_source_index.sec")
		m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		if err != nil {
			t.Fatal(err)
		}
		changed := false
		for _, fn := range m.Functions {
			if parentEnd && fn.Name == "Borrow" {
				for _, block := range fn.Blocks {
					for i := 0; i+1 < len(block.Operations); i++ {
						if block.Operations[i].Kind == OpDiscardValue && block.Operations[i+1].Kind == OpReferenceEndBorrow {
							block.Operations[i], block.Operations[i+1] = block.Operations[i+1], block.Operations[i]
							changed = true
						}
					}
				}
			}
			if !parentEnd && fn.Name == "Read" {
				for _, block := range fn.Blocks {
					for i, op := range block.Operations {
						if op.Kind == OpSliceIndexInBounds {
							branch := &block.Operations[i+1]
							branch.Successors[0], branch.Successors[1] = branch.Successors[1], branch.Successors[0]
							changed = true
						}
					}
				}
			}
		}
		if !changed {
			t.Fatal("missing mutation target")
		}
		err = Verify(m)
		if err == nil || parentEnd && !strings.Contains(err.Error(), "borrow end") {
			t.Fatalf("malformed slice access accepted: %v", err)
		}
	}
}
