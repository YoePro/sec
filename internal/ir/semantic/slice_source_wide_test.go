package semantic

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/sema"
)

// Wide scalar payloads stay independent of the target-sized slice count.
// Bounds keep original endpoint/index carriers and conservative dynamic
// origins; proven full/empty ranges and constant indexes need no extra check.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§3,13–19,25–30,126–128,140;
// rules/compiler/semantic_ir.md — §126.1–20.
func TestPackage16WideSliceSource(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program := sliceBoundaryProgram(t, "semantic_ir/slice_source_wide.sec")
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			elements, endpoints, indexes := map[string]bool{}, map[string]bool{}, map[string]bool{}
			if err := walk.Inspect(program, func(node any) error {
				switch expr := node.(type) {
				case *ast.RefExpression:
					if plan, ok := a.ResolvedSliceCreationPlanOf(expr); ok {
						elements[plan.ElementType.Name] = true
						if plan.CheckKind == sema.SliceRangeRuntimeCheck {
							if plan.StaticRange != nil || plan.DynamicRange == 0 {
								t.Fatal("runtime source range acquired an exact static origin")
							}
							endpoints[plan.Start.Type.Name], endpoints[plan.End.Type.Name] = true, true
						}
					}
				case *ast.IndexExpression:
					if plan, ok := a.ResolvedSliceIndexPlanOf(expr); ok && plan.CheckKind == sema.ArrayIndexRuntimeCheck {
						if plan.ConstantIndex != nil || plan.SliceLength != nil {
							t.Fatal("dynamic slice index acquired a constant extent/proof")
						}
						indexes[plan.IndexType.Name] = true
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"int128", "int256", "uint128", "uint256"} {
				if !elements[name] || !endpoints[name] || !indexes[name] {
					t.Fatalf("lost wide %s element/endpoint/index coverage", name)
				}
			}
			if !elements["decimal128"] {
				t.Fatal("decimal128 element coverage lost")
			}
			m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(m); err != nil {
				for _, block := range m.Functions[0].Blocks {
					for _, op := range block.Operations {
						if op.Kind == OpSliceBorrowShared || op.Kind == OpSliceReborrowShared {
							t.Logf("%s reference facts: %+v", op.Kind, op.Reference)
						}
					}
				}
				t.Fatal(err)
			}
			for _, fn := range m.Functions {
				if !fn.NoPanic {
					t.Fatal("fallible/proven wide slice source lost its noPanic proof")
				}
				ranges, indexChecks, writes := 0, 0, 0
				values := map[ValueID]TypeID{}
				for _, p := range fn.Parameters {
					values[p.Value.ID] = p.Value.Type
				}
				for _, b := range fn.Blocks {
					for _, p := range b.Parameters {
						values[p.ID] = p.Type
					}
					for _, op := range b.Operations {
						for _, result := range op.Results {
							values[result.ID] = result.Type
						}
					}
				}
				for _, b := range fn.Blocks {
					for _, op := range b.Operations {
						switch op.Kind {
						case OpSliceRangeCheck:
							ranges++
							if op.Operands[1] != fn.Parameters[1].Value.ID || op.Operands[2] != fn.Parameters[2].Value.ID {
								t.Fatal("wide endpoint SSA narrowed or reevaluated")
							}
						case OpSliceIndexInBounds:
							indexChecks++
							if op.Operands[1] != fn.Parameters[3].Value.ID {
								t.Fatal("wide index SSA narrowed or reevaluated")
							}
						case OpPlaceRead, OpPlaceWrite:
							place, _ := m.Types.Lookup(values[op.Operands[0]])
							array, _ := m.Types.Lookup(fn.Parameters[0].Value.Type)
							if place.Element != array.Element {
								t.Fatal("slice access changed the element carrier")
							}
							if op.Kind == OpPlaceWrite {
								writes++
							}
						case OpSliceLength:
							typ, _ := m.Types.Lookup(op.Results[0].Type)
							if typ.Kind != TypeUint || !typ.TargetSize {
								t.Fatal("wide element changed the target-sized unsigned count")
							}
						case OpBoundsFailure, OpArrayExtract, OpArrayReplace, OpStorageLoad:
							t.Fatal("wide slice access introduced panic or aggregate transfer", op.Kind)
						}
					}
				}
				if ranges != 1 || indexChecks != 1 {
					if strings.HasPrefix(fn.Name, "Empty") && ranges == 0 && indexChecks == 0 && writes == 0 {
						continue
					}
					t.Fatal("proven/full/empty ranges introduced redundant runtime checks", fn.Name, ranges, indexChecks)
				}
				if (fn.Name == "Signed256" || fn.Name == "Unsigned256" || fn.Name == "Decimal") && writes != 1 {
					t.Fatal("mutable wide element write lost")
				}
			}
		})
	}
}
