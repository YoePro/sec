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

// TestArenaContextsConsumeRecordedPlans verifies that analysis clients see the
// same selected contexts as successful P19 operations, independently of result
// type correlation or later analyzer profile changes. Creating an Arena never
// selects the new domain as the context providing its backing.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§52–54,72–76;
// rules/memory/allocation.md — §§5,6,17,29.
func TestArenaContextsConsumeRecordedPlans(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_operation_plans.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(data), "arena_operation_plans.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				beforeIdentities := fmt.Sprintf("%#v", a.placeIdentities)
				facts := a.AllocationContextFacts()
				checked := 0
				for _, plan := range a.arenaOperationPlans {
					allocating := plan.Operation == ArenaOperationNew || plan.Operation == ArenaOperationAlloc || plan.Operation == ArenaOperationCreate && plan.Fallible
					found := 0
					for _, fact := range facts {
						if sourceTokenLocation(fact.Source) != sourceTokenLocation(plan.Source) {
							continue
						}
						found++
						if !allocating || !fact.ContextKnown || fact.Context != plan.Context {
							t.Fatal("context differs from recorded operation plan", fact, plan)
						}
						if plan.Operation == ArenaOperationCreate {
							if fact.SelectedDomain != "" || fact.CreatedDomain != plan.Domain {
								t.Fatal("created domain became backing context", fact)
							}
						} else if fact.SelectedDomain != plan.Domain || fact.CreatedDomain != "" || !fact.Context.Available || fact.Context.Origin != StorageOriginArena {
							t.Fatal("explicit Arena selection lost", fact)
						}
					}
					if allocating {
						checked++
						if found != 1 {
							t.Fatal("missing/duplicate allocation context", plan, found)
						}
					} else if found != 0 {
						t.Fatal("nonallocating operation gained context demand", plan)
					}
				}
				if checked != len(facts) || checked == 0 {
					t.Fatal("unexpected context sites", checked, facts)
				}
				// Public reads must use recorded plans, not infer expressions or
				// reselect using the analyzer's current profile.
				a.expressionTypes = nil
				a.targetProfile = "changed-after-analysis"
				if !reflect.DeepEqual(facts, a.AllocationContextFacts()) || beforeIdentities != fmt.Sprintf("%#v", a.placeIdentities) {
					t.Fatal("context read recomputed source decisions or identities")
				}
				for call, plan := range a.arenaOperationPlans {
					if plan.Operation != ArenaOperationNew {
						continue
					}
					duplicate := *call
					plan.Context.Profile = "conflicting-context"
					a.arenaOperationPlans[&duplicate] = plan
					for run := 0; run < 4; run++ {
						for _, fact := range a.AllocationContextFacts() {
							if sourceTokenLocation(fact.Source) == sourceTokenLocation(plan.Source) && fact.ContextKnown {
								t.Fatal("conflicting plans selected a context", fact)
							}
						}
					}
					delete(a.arenaOperationPlans, &duplicate)
					break
				}
				// Unrepresented operations cannot acquire context facts from a
				// receiver spelling or their effect's domain alone.
				a.arenaOperationPlans = nil
				for _, fact := range a.AllocationContextFacts() {
					if fact.ContextKnown {
						t.Fatal("missing plan acquired a guessed context", fact)
					}
				}
			})
		}
	}
}
