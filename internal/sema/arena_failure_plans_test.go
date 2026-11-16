package sema

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Source contracts retain the same core error/result identity across all
// constructor/allocation forms, empty capacity/count, dynamic counts, scalar
// plans and analysis depths; reads cannot mutate the stored contract.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§32–35,76.
func TestArenaFailurePlans(t *testing.T) {
	for _, file := range []string{"arena_operation_plans.sec", "arena_failure_contracts.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{0, 32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", file, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(data), file))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					if errors := a.Analyze(program); len(errors) != 0 {
						t.Fatal(errors)
					}
					before := fmt.Sprintf("%#v", a.placeIdentities)
					checked, empty := 0, 0
					for call, stored := range a.arenaOperationPlans {
						plan, _ := a.ResolvedArenaOperationPlanOf(call)
						if !plan.Fallible {
							if plan.Failure != nil {
								t.Fatal("infallible operation acquired an error plan")
							}
							continue
						}
						checked++
						failure := plan.Failure
						if failure == nil || !reflect.DeepEqual(failure.ResultType, plan.ResultType) || failure.ResultType.Kind != ResultType || len(failure.ResultType.TypeArgs) != 2 || failure.ResultType.TypeArgs[1].Name != "AllocationError" || !plan.FailureAtomic {
							t.Fatal("missing stable fallible contract", plan)
						}
						if !reflect.DeepEqual(failure.ResultType.TypeArgs[1], semanticSnapshotType(a.types["AllocationError"])) {
							t.Fatal("error type differs from the canonical compiler-known AllocationError")
						}
						wantState := ArenaFailureUnchangedOwner
						if plan.Operation == ArenaOperationCreate {
							wantState = ArenaFailureNoDomain
						}
						if failure.StateOnFailure != wantState {
							t.Fatal("failed operation changes owner state", plan)
						}
						want := []ArenaFailureMapping{{ArenaFailureExhausted, "OutOfMemory"}, {ArenaFailureUnsupported, "Unsupported"}, {ArenaFailureInvalidExtent, "InvalidSize"}, {ArenaFailureInvalidAlignment, "InvalidAlignment"}}
						if !reflect.DeepEqual(failure.Mappings, want) || !reflect.DeepEqual(plan.ErrorCauses, plan.ErrorType.EnumValues) {
							t.Fatal(failure)
						}
						for _, mapping := range want {
							if got, ok := failure.ErrorVariant(mapping.Condition); !ok || got != mapping.Variant {
								t.Fatal("incorrect failure mapping", mapping, got)
							}
						}
						if got, ok := failure.ErrorVariant("unknown-provider-code"); ok || got != "" {
							t.Fatal("unknown condition silently mapped")
						}
						if plan.Count != nil && plan.Count.Sign() == 0 {
							empty++
						}
						failure.Mappings[0].Variant = "corrupt"
						failure.ResultType.TypeArgs[1].EnumValues[0] = "corrupt"
						failure.StateOnFailure = "corrupt"
						again, _ := a.ResolvedArenaOperationPlanOf(call)
						if !reflect.DeepEqual(again, stored) {
							t.Fatal("public failure query aliases stored result or mappings")
						}
					}
					if checked == 0 || empty == 0 {
						t.Fatal("fixture lost its fallible or zero-size contracts")
					}
					if before != fmt.Sprintf("%#v", a.placeIdentities) {
						t.Fatal("failure queries allocated identities")
					}
				})
			}
		}
	}
}
