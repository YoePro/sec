package sema

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Arena source plans preserve ownership, stable domains and separate epochs,
// while keeping incomplete provider/layout facts explicit. Queries detach facts
// and cannot allocate identities, infer calls or manufacture missing plans.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§4–10,32–35,70–76.
func TestArenaOperationPlans(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_operation_plans.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.New(string(data)))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				counts := map[ArenaOperationKind]int{}
				if CopyClassificationOf(a.types["Arena"]) != CopyMoveOnly || TriviallyDestructible(a.types["Arena"]) {
					t.Fatal("Arena lost builtin move-only resource semantics")
				}
				initial := map[string]EpochDependencyID{}
				for _, plan := range a.arenaOperationPlans {
					if plan.Operation == ArenaOperationCreate {
						initial[plan.Domain] = plan.EpochAfter
					}
				}
				beforeIdentities := fmt.Sprintf("%#v", a.placeIdentities)
				for call, stored := range a.arenaOperationPlans {
					plan, ok := a.ResolvedArenaOperationPlanOf(call)
					if !ok || !reflect.DeepEqual(plan, stored) {
						t.Fatalf("snapshot mismatch for %s at line %d", plan.Operation, plan.Source.Line)
					}
					counts[plan.Operation]++
					if plan.Domain == "" || plan.InvalidationDomain == 0 || plan.EpochBefore == 0 || !plan.FailureAtomic {
						t.Fatal(plan)
					}
					if plan.Operation == ArenaOperationCreate {
						if _, ok := a.ResolvedArenaCreatePlanOf(call); !ok {
							t.Fatal("missing create query")
						}
						if plan.EpochBefore != plan.EpochAfter || plan.ChangesState {
							t.Fatal("creation invents a state/epoch advance")
						}
						if plan.Constructor == "FromBuffer" {
							if plan.Fallible || plan.Backing == nil || plan.BackingRelation != BackingRelationBorrowed || plan.ReclamationAuthority != ReclamationExternalOwner {
								t.Fatal(plan)
							}
						}
					} else if plan.Receiver == nil {
						t.Fatal("missing resolved receiver Place")
					}
					if plan.Fallible {
						if plan.ResultType.Kind != ResultType || plan.ErrorType.Name != "AllocationError" || !reflect.DeepEqual(plan.ErrorCauses, plan.ErrorType.EnumValues) {
							t.Fatal(plan)
						}
					}
					switch plan.Operation {
					case ArenaOperationNew, ArenaOperationAlloc:
						if _, ok := a.ResolvedArenaAllocationPlanOf(call); !ok {
							t.Fatal("missing allocation query")
						}
						if plan.EpochBefore != plan.EpochAfter || !plan.ChangesState || plan.ConsumesOwner || !plan.Context.Available || plan.Context.Origin != StorageOriginArena {
							t.Fatal(plan)
						}
						if !plan.ElementDefaultable || !plan.ElementTriviallyDestructible || plan.SuccessType.Kind != ReferenceType || !plan.SuccessType.ReferenceMutable || len(plan.Unresolved) == 0 {
							t.Fatal(plan)
						}
						if plan.Count != nil && plan.Count.Sign() == 0 {
							if plan.Allocation != 0 || !plan.Fallible {
								t.Fatal("zero count changes contract or invents allocation")
							}
						}
						if plan.CountUnit != "elements" {
							t.Fatal("allocation count became byte capacity")
						}
					case ArenaOperationReset:
						if _, ok := a.ResolvedArenaResetPlanOf(call); !ok {
							t.Fatal("missing reset query")
						}
						if plan.EpochAfter == plan.EpochBefore || plan.EndsDomain || plan.ConsumesOwner {
							t.Fatal(plan)
						}
					case ArenaOperationRelease:
						if _, ok := a.ResolvedArenaReleasePlanOf(call); !ok {
							t.Fatal("missing release query")
						}
						if !plan.EndsDomain || !plan.ConsumesOwner || plan.EpochAfter != 0 {
							t.Fatal("release invents another live epoch")
						}
					}
					if plan.Operation == ArenaOperationNew && plan.ElementType.Name == "uint256" && plan.Constructor != "WithCapacity" {
						t.Fatal("owner move lost creation provenance")
					}
					if plan.Count != nil {
						plan.Count.SetInt64(999)
					}
					if plan.Backing != nil {
						plan.Backing.Root = "corrupt"
						if plan.Backing.Type.Element != nil {
							plan.Backing.Type.Element.Name = "corrupt"
						}
					}
					if plan.Receiver != nil {
						plan.Receiver.Root = "corrupt"
					}
					if len(plan.ErrorCauses) > 0 {
						plan.ErrorCauses[0] = "corrupt"
					}
					if len(plan.Unresolved) > 0 {
						plan.Unresolved[0] = "corrupt"
					}
					plan.ResultType.Name = "corrupt"
					again, _ := a.ResolvedArenaOperationPlanOf(call)
					if !reflect.DeepEqual(again, stored) {
						t.Fatal("query aliases plan")
					}
				}
				want := map[ArenaOperationKind]int{ArenaOperationCreate: 3, ArenaOperationNew: 2, ArenaOperationAlloc: 4, ArenaOperationReset: 2, ArenaOperationRelease: 3}
				if !reflect.DeepEqual(counts, want) {
					t.Fatalf("counts=%v, want %v", counts, want)
				}
				var dynamicUses int
				for _, use := range a.resolvedReferenceUses {
					if use.Facts.Provenance == ReferenceProvenanceArena {
						dynamicUses++
						matched := false
						for _, producer := range a.arenaOperationPlans {
							if (producer.Operation == ArenaOperationNew || producer.Operation == ArenaOperationAlloc) && producer.InvalidationDomain == use.Facts.InvalidationDomain && producer.EpochAfter == use.Facts.EpochDependency {
								matched = true
							}
						}
						if !matched {
							t.Fatal("reference use and allocation plan disagree on canonical epoch")
						}
						if use.Facts.Validity != ReferenceValidityDynamicEpoch || use.Facts.InvalidationDomain == 0 || use.Facts.EpochDependency == 0 {
							t.Fatal("allocation reference lost canonical epoch facts")
						}
					}
				}
				if dynamicUses == 0 {
					t.Fatal("no source dynamic-epoch use")
				}
				if len(initial) != 3 {
					t.Fatal("distinct creations share a domain")
				}
				if _, ok := a.ResolvedArenaOperationPlanOf(&ast.CallExpression{}); ok {
					t.Fatal("query inferred a call")
				}
				if beforeIdentities != fmt.Sprintf("%#v", a.placeIdentities) {
					t.Fatal("query allocated identity")
				}
				if err := a.ValidateAllocationLoweringReadiness(program, "main"); err == nil {
					t.Fatal("plan metadata opened readiness without IR")
				}
				old := a.arenaOperationPlans
				a.Analyze(&ast.Program{})
				for call := range old {
					if _, ok := a.ResolvedArenaOperationPlanOf(call); ok {
						t.Fatal("stale plan survived reanalysis")
					}
				}
			})
		}
	}
}

// Rejected element/count/authority/dependency forms publish no operation plan.
// Rules: rules/memory/arena.md — §§19–25,35–46;
// rules/mlir/packages/sec-mlir-dialect_package19.md — §§71–76.
func TestArenaOperationPlansRejectInvalid(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_operation_plans_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	for _, plan := range a.arenaOperationPlans {
		if plan.Operation != ArenaOperationCreate && plan.Operation != ArenaOperationNew {
			t.Fatal("invalid call published plan", plan)
		}
	}
	if len(a.arenaOperationPlans) != 4 {
		t.Fatalf("plan count %d, expected three valid constructors and one valid allocation", len(a.arenaOperationPlans))
	}
	var missing *Analyzer
	if _, ok := missing.ResolvedArenaOperationPlanOf(nil); ok {
		t.Fatal("nil analyzer has a plan")
	}
}
