package semantic

import (
	"fmt"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/sema"
)

// The two resolved core spellings observe the same non-consuming uint count
// for shared/mutable, nested and empty slices, including an inline operand.
// Rules: rules/compiler/semantic_ir.md — §126.14;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,63.
func TestPackage16SliceSourceLength(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, _ := sliceBoundaryInput(t, "semantic_ir/slice_source_length.sec")
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			for _, fn := range m.Functions {
				lengths, guards, copies, reads, ends := 0, 0, 0, 0, 0
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						switch op.Kind {
						case OpSliceLength:
							lengths++
							typ, _ := m.Types.Lookup(op.Results[0].Type)
							if typ.Kind != TypeUint || typ.Name != "uint" || !typ.TargetSize {
								t.Fatal("length was narrowed or signed")
							}
						case OpReferenceIsValid:
							guards++
						case OpSliceCopyShared, OpSliceMoveMutable:
							copies++
						case OpPlaceRead, OpArrayExtract, OpStorageLoad:
							reads++
						case OpReferenceEndBorrow:
							ends++
						}
					}
				}
				if guards != 0 || copies != 0 || reads != 0 {
					t.Fatalf("%s: introduced guard/copy/read %d/%d/%d", fn.Name, guards, copies, reads)
				}
				if fn.Name == "Both" && lengths != 5 {
					t.Fatalf("both core forms and reborrow extent: %d", lengths)
				}
				if fn.Name == "Mutable" && lengths != 2 {
					t.Fatal("mutable source consumed or omitted")
				}
				if fn.Name == "Inline" && (lengths != 1 || ends != 1) {
					t.Fatal("inline operand lifetime lost")
				}
			}
		})
	}
}

// Signature lowering remains separate: install a canonical slice parameter
// as white-box input to the actual length source producer for both core forms
// and authorities. Epoch-injected evidence is distinct from Arena source IR.
// Rules: rules/compiler/semantic_ir.md — §126.14,18;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45,47–49,136.
func TestPackage16LengthTemporalProducer(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		program, a := sliceBoundaryInput(t, "semantic_ir/slice_length_forms.sec")
		for _, statement := range program.Statements {
			decl, ok := statement.(*ast.FunctionDeclaration)
			if !ok {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s", width, decl.Name.Value), func(t *testing.T) {
				m := &Module{Version: Version, Identity: "main", Types: NewTypeTable(), PointerWidthBits: width}
				b := &builder{module: m, analyzer: a, maxPackage: 16}
				if err := b.prepareSliceLowering(program); err != nil {
					t.Fatal(err)
				}
				resolved, _ := a.ResolvedFunctionForDeclaration(decl)
				parameter := resolved.Parameters[0]
				facts, ok := a.ReferenceParameterFactsOf(parameter)
				if !ok {
					t.Fatal("missing canonical reference facts")
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
				uintType := m.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
				fn := &Function{ID: FunctionID("main::" + decl.Name.Value), Name: decl.Name.Value, ReturnType: uintType}
				m.Functions = []*Function{fn}
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
				if err := fb.buildLet(decl.Body.Statements[0].(*ast.LetStatement)); err != nil {
					t.Fatal(err)
				}
				if err := fb.buildReturn(decl.Body.Statements[1].(*ast.ReturnStatement)); err != nil {
					t.Fatal(err)
				}
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				entry, success := fn.Blocks[0], fn.Blocks[1]
				if len(entry.Operations) != 2 || entry.Operations[0].Kind != OpReferenceIsValid || success.Operations[0].Kind != OpSliceLength || success.Operations[0].ReferenceGuard != entry.Operations[0].Results[0].ID {
					t.Fatal("length source producer lost temporal guard")
				}
				success.Operations[0].ReferenceGuard = 0
				if err := Verify(m); err == nil {
					t.Fatal("unguarded dynamic source observation accepted")
				}
			})
		}
	}
}

// Replacing a resolved core expression with a fresh unresolved AST node must
// fail without a partial module; the backend cannot rediscover len by name.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,47–49,128.
func TestPackage16MissingLengthFacts(t *testing.T) {
	for _, name := range []string{"Property", "Global"} {
		program, a := sliceBoundaryInput(t, "semantic_ir/slice_source_length.sec")
		for _, statement := range program.Statements {
			decl, ok := statement.(*ast.FunctionDeclaration)
			if !ok || decl.Name.Value != name {
				continue
			}
			binding := decl.Body.Statements[1].(*ast.LetStatement)
			switch expr := binding.Value.(type) {
			case *ast.MemberExpression:
				fresh := *expr
				binding.Value = &fresh
			case *ast.CallExpression:
				fresh := *expr
				binding.Value = &fresh
			}
		}
		m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		if m != nil || err == nil {
			t.Fatalf("%s lost semantic resolution but produced IR: %v", name, err)
		}
	}
}
