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

// Dynamic membership observes the canonical owner with guarded reads and a
// short-circuit scalar search, without owner copies, transfers or allocations.
// The runtime length, rather than capacity, controls the empty/absent exit.
// Rules: rules/foundations/operators.md — §67;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§40-45,110.
func TestDynamicArrayMembershipIR(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/dynamic_membership/scalars.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "dynamic-membership.sec"))
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
		searches := 0
		for _, fn := range module.Functions {
			if fn.Name == "Needle" {
				continue
			}
			searches++
			reads, lengths, negations, calls := 0, 0, 0, 0
			seenCall := false
			definitions := map[ValueID]Operation{}
			for _, block := range fn.Blocks {
				for i := range block.Operations {
					op := &block.Operations[i]
					for _, value := range op.Results {
						definitions[value.ID] = *op
					}
					switch op.Kind {
					case OpDynamicArrayAllocate, OpDynamicArrayGrowBacking, OpDynamicArrayAppendPrepared, OpOwnSemanticCopy, OpOwnMove, OpOwnMoveFromPlace:
						t.Fatalf("%s: membership allocated/transferred an owner with %s", fn.Name, op.Kind)
					case OpDirectCall:
						calls++
						seenCall = true
					case OpDynamicArrayLength:
						lengths++
						if fn.Name == "Effects" && !seenCall {
							t.Fatal("array observed before needle")
						}
					case OpBoolNot:
						negations++
					case OpFloatCompare:
						if op.IntegerCompare != IntegerCompareEQ {
							t.Fatal("changed NaN equality")
						}
					case OpDynamicArrayElementPlace:
						reads++
						if op.ArrayCheckKind != ArrayIndexRuntimeCheck || op.ArrayProofKind != ArrayIndexProofGuarded || op.ArrayGuard == 0 || op.Place == nil || op.Place.BackingDomain == 0 {
							t.Fatal("missing bounded backing read")
						}
						guard := op.ArrayGuard
						op.ArrayGuard = 0
						if err := Verify(module); err == nil {
							t.Fatal("unguarded search read verified")
						}
						op.ArrayGuard = guard
						domain := op.Place.BackingDomain
						op.Place.BackingDomain = 0
						if err := Verify(module); err == nil {
							t.Fatal("search detached from backing domain")
						}
						op.Place.BackingDomain = domain
					}
				}
			}
			wantLengths := 1
			if fn.Name == "Contains" {
				wantLengths = 2
			} // The source owner is reused after search.
			if reads != 1 || lengths != wantLengths {
				t.Fatalf("%s: reads=%d lengths=%d", fn.Name, reads, lengths)
			}
			if fn.Name == "Effects" && calls != 1 {
				t.Fatal("needle evaluated more than once")
			}
			if fn.Name == "Absent" && negations != 1 {
				t.Fatal("not in did not negate one completed search")
			}
			emptyExit := false
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind != OpCondBranch || len(op.Operands) != 1 || len(op.Successors) != 2 {
						continue
					}
					predicate := definitions[op.Operands[0]]
					if predicate.Kind == OpIntCompare && predicate.IntegerCompare == IntegerCompareLT && len(predicate.Operands) == 2 && definitions[predicate.Operands[1]].Kind == OpDynamicArrayLength {
						exit := op.Successors[1]
						if len(exit.Arguments) != 1 || definitions[exit.Arguments[0]].Bool == nil || *definitions[exit.Arguments[0]].Bool {
							t.Fatal("empty search does not exit false")
						}
						emptyExit = true
					}
				}
			}
			if !emptyExit {
				t.Fatalf("%s has no length-bounded short-circuit exit", fn.Name)
			}
		}
		if searches != 8 {
			t.Fatal("missing search forms", searches)
		}
		if _, err := Serialize(module); err != nil {
			t.Fatal(err)
		}
	}
}

// Unsupported equality and fresh owners remain explicit boundaries rather
// than ordinary equality, an owner copy or invented temporary lifetime.
// Rules: rules/foundations/operators.md — §67;
// rules/compiler/compiler_pipeline.md — unsupported implementation boundaries.
func TestDynamicArrayMembershipBoundaries(t *testing.T) {
	for _, test := range []struct{ file, want string }{
		{"string.sec", "dynamic-array membership element equality"},
		{"temporary.sec", "dynamic-array membership without resolved owner/backing-domain facts"},
	} {
		program, analyzer := sliceBoundaryInput(t, "semantic_ir/dynamic_membership/"+test.file)
		if module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 18}); err == nil || module != nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: module=%v err=%v", test.file, module != nil, err)
		}
	}
}
