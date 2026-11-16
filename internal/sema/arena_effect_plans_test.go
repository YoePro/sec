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

// Every successful source operation must retain its existing ordered event,
// canonical resource and transition, including provider uncertainty. Equivalent
// allocations stay separate across resets and independent Arena resources.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§65–69,76;
// rules/memory/arena.md — §§76–79.
func TestArenaOperationEffectPlans(t *testing.T) {
	for _, file := range []string{"arena_operation_plans.sec", "arena_effect_plans.sec"} {
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
					identities := fmt.Sprintf("%#v", a.placeIdentities)
					indices := map[CallableID]map[int]bool{}
					for call, stored := range a.arenaOperationPlans {
						plan, _ := a.ResolvedArenaOperationPlanOf(call)
						effect := plan.Effects
						if effect == nil || effect.Resource != plan.InvalidationDomain || !effect.MustPreserveOrder || effect.CSESafe {
							t.Fatal("missing resource/ordering barrier", plan)
						}
						sequence := a.CallGraph().ArenaSummary(effect.Callable).DirectEffects
						if effect.DirectEffectIndex < 0 || effect.DirectEffectIndex >= len(sequence) || !reflect.DeepEqual(effect.Event, sequence[effect.DirectEffectIndex]) || effect.Event.Domain != plan.Domain || effect.Event.Source != plan.Source {
							t.Fatal("effect plan does not identify its call graph event", effect)
						}
						if indices[effect.Callable] == nil {
							indices[effect.Callable] = map[int]bool{}
						}
						if indices[effect.Callable][effect.DirectEffectIndex] {
							t.Fatal("distinct operations share one direct event")
						}
						indices[effect.Callable][effect.DirectEffectIndex] = true
						wantKind, wantTransition := ArenaEffectAllocate, BackingTransitionNone
						wantProvider := ArenaProviderNone
						switch plan.Operation {
						case ArenaOperationCreate:
							wantTransition = BackingTransitionEstablishDomain
							wantKind = ArenaEffectCreateBorrowed
							if plan.Constructor == "WithCapacity" {
								wantKind, wantProvider = ArenaEffectCreateOwned, ArenaProviderAcquire
							} else if plan.Constructor == "Growable" {
								wantKind, wantProvider = ArenaEffectCreateGrowable, ArenaProviderAcquire
							}
						case ArenaOperationNew, ArenaOperationAlloc:
							if plan.Count == nil || plan.Count.Sign() != 0 {
								if plan.GrowthKind == "growable-nonrelocating" {
									wantProvider = ArenaProviderGrow
								} else if plan.GrowthKind == "" {
									wantProvider = ArenaProviderFromBacking
								}
							}
						case ArenaOperationReset:
							wantKind, wantTransition = ArenaEffectReset, BackingTransitionAdvanceEpoch
						case ArenaOperationRelease:
							wantKind, wantTransition = ArenaEffectRelease, BackingTransitionEndDomain
							if plan.BackingRelation == BackingRelationOwned {
								wantProvider = ArenaProviderReclaim
							}
							if effect.EndsBackingBorrow != (plan.BackingRelation == BackingRelationBorrowed) {
								t.Fatal("borrowed Release/provider reclamation conflated")
							}
						}
						wantAllocate := wantKind == ArenaEffectAllocate || wantKind == ArenaEffectCreateOwned || wantKind == ArenaEffectCreateGrowable
						if effect.Event.Kind != wantKind || effect.Event.MayAllocate != wantAllocate || effect.Transition != wantTransition || effect.Provider != wantProvider || effect.ProviderEffectsUnknown != (wantProvider != ArenaProviderNone) {
							t.Fatal("incorrect effect contract", effect)
						}
						effect.Event.Domain = "corrupt"
						effect.Resource = 0
						effect.ProviderEffectsUnknown = false
						effect.MustPreserveOrder, effect.CSESafe = false, true
						again, _ := a.ResolvedArenaOperationPlanOf(call)
						if !reflect.DeepEqual(again, stored) {
							t.Fatal("public effect query aliases stored plan")
						}
					}
					for callable, seen := range indices {
						if len(seen) != len(a.CallGraph().ArenaSummary(callable).DirectEffects) {
							t.Fatal("operation plans lost/duplicated a direct effect")
						}
						if a.callGraph.nodes[callable].Name == "Order" {
							sequence := a.CallGraph().ArenaSummary(callable).DirectEffects
							want := []ArenaEffectKind{ArenaEffectCreateBorrowed, ArenaEffectCreateBorrowed,
								ArenaEffectAllocate, ArenaEffectAllocate, ArenaEffectReset, ArenaEffectReset,
								ArenaEffectAllocate, ArenaEffectRelease, ArenaEffectRelease}
							if len(sequence) != len(want) {
								t.Fatal("source effect sequence changed", sequence)
							}
							for i, kind := range want {
								if sequence[i].Kind != kind {
									t.Fatal("allocation moved across an invalidation", sequence)
								}
							}
							left, right := sequence[0].Domain, sequence[1].Domain
							if left == right || sequence[2].Domain != left || sequence[3].Domain != left || sequence[4].Domain != right || sequence[5].Domain != left || sequence[6].Domain != left || sequence[7].Domain != left || sequence[8].Domain != right {
								t.Fatal("independent Arena resources conflated", sequence)
							}
						}
					}
					if identities != fmt.Sprintf("%#v", a.placeIdentities) {
						t.Fatal("effect queries allocated identities")
					}
				})
			}
		}
	}
}
