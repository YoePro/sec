package semantic

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/sema"
)

// The real source producer evaluates endpoints before temporal validation,
// then observes length/checks the range and forms a guarded reborrow. The
// parent parameter's epoch facts are injected as white-box IR evidence while
// source signature/Arena allocation production remains P16-36/P18 work. Both
// ordinary and fallible spatial checks leave stale-reference failure intact.
// Rules: rules/compiler/semantic_ir.md — §126.8,18;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,43–46,136.
func TestPackage16ReborrowTemporalProducer(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, a := sliceBoundaryInput(t, "semantic_ir/slice_reborrow_temporal.sec")
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
				if !strings.HasPrefix(decl.Name.Value, "Temporal") {
					if err := b.buildFunction(decl); err != nil {
						t.Fatal(err)
					}
					continue
				}
				resolved, ok := a.ResolvedFunctionForDeclaration(decl)
				if !ok {
					t.Fatal("missing function")
				}
				facts, ok := a.ReferenceParameterFactsOf(resolved.Parameters[0])
				if !ok {
					t.Fatal("missing parameter facts")
				}
				facts.Validity, facts.ProofProvenance = sema.ReferenceValidityDynamicEpoch, ""
				facts.InvalidationDomain, facts.EpochDependency = 7, 9
				element, err := b.internType(*resolved.Parameters[0].Type.Element.Element)
				if err != nil {
					t.Fatal(err)
				}
				slice, err := m.Types.InternSlice(element, false)
				if err != nil {
					t.Fatal(err)
				}
				void := m.Types.Intern(Type{Kind: TypeVoid, Name: "void"})
				fn := &Function{ID: FunctionID("main::" + decl.Name.Value), Name: decl.Name.Value, ReturnType: void}
				m.Functions = append(m.Functions, fn)
				fb := &functionBuilder{owner: b, fn: fn, bindings: map[sema.BindingID]binding{}}
				fb.current = fb.newBlock()
				value := fb.newValue(slice, OwnershipSharedReference, Location{})
				metadata := referenceMetadataOf(facts)
				fn.Parameters = []Parameter{{Name: "view", Value: value, Reference: metadata}}
				bindingFact, ok := a.ResolvedBindingOf(decl.Parameters[0].Name)
				if !ok {
					t.Fatal("missing parameter binding")
				}
				fb.bindings[bindingFact.ID] = binding{value: value.ID, typ: slice}
				fb.recordReference(value.ID, metadata)
				creation := decl.Body.Statements[0].(*ast.LetStatement)
				protected := creation.Value
				if tried, ok := protected.(*ast.TryExpression); ok {
					protected = tried.Expression
				}
				ref := protected.(*ast.RefExpression)
				plan := b.slicePlans.creations[ref]
				plan.ReferenceFacts.Facts.Validity, plan.ReferenceFacts.Facts.ProofProvenance = facts.Validity, ""
				plan.ReferenceFacts.Facts.InvalidationDomain, plan.ReferenceFacts.Facts.EpochDependency = facts.InvalidationDomain, facts.EpochDependency
				b.slicePlans.creations[ref] = plan
				if err := fb.buildLet(creation); err != nil {
					t.Fatal(err)
				}
				if err := fb.buildDiscard(decl.Body.Statements[1].(*ast.DiscardStatement)); err != nil {
					t.Fatal(err)
				}
				fb.emit(Operation{Kind: OpReturn})
				entry := fn.Blocks[0].Operations
				if len(entry) != 4 || entry[0].Kind != OpDirectCall || entry[1].Kind != OpDirectCall || entry[2].Kind != OpReferenceIsValid || entry[3].Kind != OpCondBranch {
					t.Fatal("endpoints were not evaluated once before validation")
				}
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			for _, fn := range m.Functions {
				if !strings.HasPrefix(fn.Name, "Temporal") {
					continue
				}
				for _, block := range fn.Blocks {
					for index := range block.Operations {
						op := &block.Operations[index]
						if fn.Name == "TemporalFallible" && op.Kind == OpBoundsFailure {
							t.Fatal("fallible spatial check panicked instead of routing RangeError")
						}
						if op.Kind == OpReferenceGenerationFailure {
							op.Kind, op.ArrayOperation = OpBoundsFailure, "slice-range"
							op.PanicReasonID, op.PanicReason = 0, ""
						}
					}
				}
			}
			if err := Verify(m); err == nil || !strings.Contains(err.Error(), "fail.reference-generation") {
				t.Fatalf("stale failure became spatial: %v", err)
			}
		})
	}
}
