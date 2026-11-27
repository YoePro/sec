package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Shared/mutable owner observations use canonical reference facts and retain
// guarded indexing and backing dependencies without copying the array.
// Rules: rules/foundations/operators.md — §67;
// rules/memory/reference_model.md — reference-mediated Place access;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§40-45,49-50.
func TestDynamicArrayReferenceObservations(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/dynamic_references/observe.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "dynamic-reference.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := analyzer.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 18})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		if len(module.Functions) != 7 {
			t.Fatal("missing observation forms")
		}
		for _, fn := range module.Functions {
			derefs, elements, lengths, writes := 0, 0, 0, 0
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					switch op.Kind {
					case OpReferenceDerefPlace:
						derefs++
						if op.Reference == nil {
							t.Fatal("dereference lost validity facts")
						}
					case OpDynamicArrayLength:
						lengths++
					case OpDynamicArrayElementPlace:
						elements++
						if op.Place == nil || op.Place.BackingDomain == 0 || op.ArrayGuard == 0 {
							t.Fatal("reference index lost bounds/backing facts")
						}
						typ, _ := module.Types.Lookup(op.Results[0].Type)
						if typ.Mutable != (fn.Name == "Write" || fn.Name == "Absent") {
							t.Fatalf("%s: changed reference authority", fn.Name)
						}
					case OpPlaceWrite:
						writes++
					case OpOwnSemanticCopy, OpDynamicArrayAllocate, OpDynamicArrayGrowBacking:
						t.Fatalf("%s: observation copied or allocated owner", fn.Name)
					}
				}
			}
			wantDerefs, wantElements, wantLengths, wantWrites := 1, 1, 1, 0
			switch fn.Name {
			case "Contains":
				wantDerefs, wantLengths = 2, 2
			case "Length", "Empty":
				wantElements = 0
			case "Read":
				wantLengths = 0
			case "Write":
				wantLengths, wantWrites = 0, 1
			}
			if derefs != wantDerefs || elements != wantElements || lengths != wantLengths || writes != wantWrites {
				t.Fatalf("%s: dereferences=%d elements=%d lengths=%d writes=%d", fn.Name, derefs, elements, lengths, writes)
			}
		}
		if _, err := Serialize(module); err != nil {
			t.Fatal(err)
		}
	}
}

// Opaque reference parameters must not borrow the holder's storage identity
// as the dynamic owner's backing identity.
// Rules: rules/compiler/semantic_ir.md — canonical resolved facts;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§40-45.
func TestDynamicArrayReferenceUnknownOwnerBoundary(t *testing.T) {
	program, analyzer := sliceBoundaryInput(t, "semantic_ir/dynamic_references/opaque.sec")
	module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 18})
	if err == nil || module != nil || !strings.Contains(err.Error(), "without resolved owner/backing-domain facts") {
		t.Fatalf("opaque reference invented owner facts: module=%v err=%v", module != nil, err)
	}
}
