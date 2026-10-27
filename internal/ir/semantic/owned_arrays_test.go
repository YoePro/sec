package semantic

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Whole fixed arrays use existing ownership and compact destruction plans;
// element moves stay distinct from forbidden indexed partial extraction.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 15, 65–67;
// rules/memory/destruction.md — § 7.3.
func TestOwnedFixedArrays(t *testing.T) {
	program, analyzer := sliceBoundaryInput(t, "semantic_ir/owned_fixed_arrays.sec")
	before := analyzer.ResolvedTransferPlans()
	module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, analyzer.ResolvedTransferPlans()) {
		t.Fatal("building arrays changed Sema plans")
	}
	arrayPlans := 0
	for _, plan := range module.DestructionPlans {
		if plan.Kind != "fixed-array" {
			continue
		}
		arrayPlans++
		typ, ok := module.Types.Lookup(plan.Type)
		if !ok || plan.ElementCount != typ.Length || plan.Element == "" || len(plan.Fields) != 0 || len(plan.Variants) != 0 {
			t.Fatalf("non-compact or incomplete array plan: %+v", plan)
		}
	}
	if arrayPlans != 4 {
		t.Fatalf("array plans = %d, want four distinct shapes", arrayPlans)
	}
	counts := map[OpKind]int{}
	movedElements := 0
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				counts[op.Kind]++
				if op.Kind == OpArrayConstruct {
					if op.Results[0].Ownership != OwnershipOwned {
						t.Fatal("array construction lost ownership")
					}
					for _, action := range op.ArrayActions {
						if action == ArrayActionMove {
							movedElements++
						}
					}
				}
			}
			if fn.Name == "Large" && len(block.Operations) > 15 {
				t.Fatal("million-element default expanded into individual elements")
			}
		}
	}
	if movedElements != 4 || counts[OpArrayConstruct] != 7 || counts[OpArrayDefault] != 3 || counts[OpOwnReplacePlace] == 0 || counts[OpPlaceArrayElement] == 0 || counts[OpDiscardValue] == 0 {
		t.Fatalf("missing producer/borrow coverage: moved=%d, ops=%v", movedElements, counts)
	}
	second, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil || Format(second) != Format(module) {
		t.Fatalf("non-deterministic array IR: %v", err)
	}
}

// Verification must reject forged copying, repeated consumption, and stale
// use after ownership was transferred into an array.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 15, 65–67, 79.
func TestOwnedArrayConstructRejectsInvalidTransfers(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"copy", "cannot copy non-trivial element"},
		{"twice", "after array.construct consumed"},
		{"use-after", "after array.construct consumed"},
		{"spread", "spread requires copy-trivial elements"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, analyzer := sliceBoundaryInput(t, "semantic_ir/owned_fixed_arrays.sec")
			module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			for _, fn := range module.Functions {
				if tc.name == "spread" && fn.Name == "Relay" {
					typ, _ := module.Types.Lookup(fn.Parameters[0].Value.Type)
					op := Operation{Kind: OpArrayConstruct, ArrayElementType: typ.Element, ArrayLength: typ.Length,
						Operands: []ValueID{fn.Parameters[0].Value.ID}, ArraySegmentKinds: []ArrayConstructSegmentKind{ArraySegmentSpread},
						ArraySegmentLengths: []string{typ.Length}, ArrayActions: []ArrayTransferAction{ArrayActionCopyTrivial},
						Results: []Value{{ID: 999999, Type: fn.Parameters[0].Value.Type, Ownership: OwnershipOwned}}}
					fn.Blocks[0].Operations = append([]Operation{op}, fn.Blocks[0].Operations...)
				}
				if fn.Name != "Build" || tc.name == "spread" {
					continue
				}
				for _, block := range fn.Blocks {
					for index := range block.Operations {
						op := &block.Operations[index]
						if op.Kind != OpArrayConstruct {
							continue
						}
						switch tc.name {
						case "copy":
							op.ArrayActions[0] = ArrayActionCopyTrivial
						case "twice":
							op.Operands[1] = op.Operands[0]
						case "use-after":
							value := op.Operands[0]
							definition, _ := structDefinition(module, op.ArrayElementType)
							read := Operation{Kind: OpStructExtractField, Operands: []ValueID{value}, StructField: 0,
								StructActions: []StructFieldAction{StructActionCopyTrivial}, Results: []Value{{ID: 999999, Type: definition.Fields[0].Type, Ownership: OwnershipImmediate}}}
							block.Operations = append(block.Operations[:index+1], append([]Operation{read}, block.Operations[index+1:]...)...)
						}
						break
					}
				}
			}
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Verify = %v, want %q", err, tc.want)
			}
		})
	}
}

// Unrepresented partial-construction cleanup returns no partial module.
// Rules: rules/memory/destruction.md — § 14.2; P17 — §§ 14, 65.
func TestOwnedArrayDeferredProducerBoundaries(t *testing.T) {
	for _, tc := range []struct{ fixture, feature string }{
		{"owned_fixed_arrays_partial.sec", "P17-16"},
		{"owned_fixed_array_temporary.sec", "P17-40"},
		{"owned_fixed_array_observation.sec", "P17-40"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			program, analyzer := sliceBoundaryInput(t, "semantic_ir/"+tc.fixture)
			module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 17})
			var unsupported *UnsupportedFeatureError
			if module != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, tc.feature) || unsupported.Location.Line <= 0 {
				t.Fatalf("array boundary: module=%v err=%v", module != nil, err)
			}
		})
	}
}
