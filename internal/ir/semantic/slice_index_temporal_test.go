package semantic

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Parameter signatures remain P16-36. Epoch-qualified slice parameters are
// installed as white-box input to the real source producer, proving index
// evaluation -> validity -> bounds -> element Place -> RHS order. A handled
// spatial failure cannot catch a stale reference or bypass validation.
// Rules: rules/compiler/semantic_ir.md — §126.15–20,25;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45–46,50–58,136.
func TestPackage16SliceIndexTemporalProducer(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, name := range []string{"Read", "Write", "Fallible"} {
			t.Run(fmt.Sprintf("%d/%s", width, name), func(t *testing.T) {
				program, a := sliceBoundaryInput(t, "semantic_ir/slice_index_temporal.sec")
				m := &Module{Version: Version, Identity: "main", Types: NewTypeTable(), PointerWidthBits: width}
				b := &builder{module: m, analyzer: a, maxPackage: 16, definedEnums: map[TypeID]bool{}, definedUnions: map[TypeID]bool{}, definedStructs: map[TypeID]bool{}}
				if err := b.prepareSliceLowering(program); err != nil {
					t.Fatal(err)
				}
				for _, statement := range program.Statements {
					decl, ok := statement.(*ast.FunctionDeclaration)
					if !ok {
						continue
					}
					if decl.Name.Value == "Index" || decl.Name.Value == "Replacement" {
						if err := b.buildFunction(decl); err != nil {
							t.Fatal(err)
						}
						continue
					}
					if decl.Name.Value != name {
						continue
					}
					resolved, _ := a.ResolvedFunctionForDeclaration(decl)
					parameter := resolved.Parameters[0]
					facts, ok := a.ReferenceParameterFactsOf(parameter)
					if !ok {
						t.Fatal("missing parameter facts")
					}
					facts.Validity, facts.ProofProvenance = sema.ReferenceValidityDynamicEpoch, ""
					facts.InvalidationDomain, facts.EpochDependency = 4, 7
					element, err := b.internType(*parameter.Type.Element.Element)
					if err != nil {
						t.Fatal(err)
					}
					slice, err := m.Types.InternSlice(element, parameter.Type.ReferenceMutable)
					if err != nil {
						t.Fatal(err)
					}
					void := m.Types.Intern(Type{Kind: TypeVoid, Name: "void"})
					fn := &Function{ID: FunctionID("main::" + name), Name: name, ReturnType: void}
					m.Functions = append(m.Functions, fn)
					fb := &functionBuilder{owner: b, fn: fn, bindings: map[sema.BindingID]binding{}}
					fb.current = fb.newBlock()
					ownership := OwnershipSharedReference
					if parameter.Type.ReferenceMutable {
						ownership = OwnershipMutableReference
					}
					value := fb.newValue(slice, ownership, Location{})
					metadata := referenceMetadataOf(facts)
					fn.Parameters = []Parameter{{Name: "view", Value: value, Reference: metadata}}
					bindingFact, _ := a.ResolvedBindingOf(decl.Parameters[0].Name)
					fb.bindings[bindingFact.ID] = binding{value: value.ID, typ: slice}
					fb.recordReference(value.ID, metadata)
					if err := fb.buildStatements(decl.Body.Statements); err != nil {
						t.Fatal(err)
					}
					fb.emit(Operation{Kind: OpReturn})
					entry := fn.Blocks[0].Operations
					if len(entry) != 3 || entry[0].Kind != OpDirectCall || entry[1].Kind != OpReferenceIsValid || entry[2].Kind != OpCondBranch {
						t.Fatal("index was not evaluated once before temporal validation")
					}
					spatial := fn.Blocks[entry[2].Successors[0].Block]
					if len(spatial.Operations) != 2 || spatial.Operations[0].Kind != OpSliceIndexInBounds || spatial.Operations[0].ReferenceGuard != entry[1].Results[0].ID {
						t.Fatal("bounds check lost its temporal guard")
					}
					access := fn.Blocks[spatial.Operations[1].Successors[0].Block]
					if access.Operations[0].Kind != OpSliceElementPlace || name == "Write" && access.Operations[1].Kind != OpDirectCall {
						t.Fatal("RHS precedes the guarded destination Place")
					}
				}
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				for _, fn := range m.Functions {
					if fn.Name != name {
						continue
					}
					for _, block := range fn.Blocks {
						for i := range block.Operations {
							op := &block.Operations[i]
							if name == "Fallible" && op.Kind == OpBoundsFailure {
								t.Fatal("handled spatial failure panicked")
							}
							if op.Kind == OpReferenceGenerationFailure {
								op.Kind, op.ArrayOperation = OpBoundsFailure, "slice-index"
								op.PanicReasonID, op.PanicReason = 0, ""
							}
						}
					}
				}
				if err := Verify(m); err == nil || !strings.Contains(err.Error(), "fail.reference-generation") {
					t.Fatalf("spatial failure replaced temporal failure: %v", err)
				}
			})
		}
	}
}
