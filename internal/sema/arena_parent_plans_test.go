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

// Source snapshots retain named/temporary backing, child/parent owner moves,
// grandchildren, branch/loop scope cleanup, zero-length backing and epochs.
// Queries must detach these facts and never allocate identities or rerun analysis.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§49–50,76;
// rules/memory/arena.md — §51.
func TestArenaParentOperationPlans(t *testing.T) {
	for _, file := range []string{"arena_nested_valid.sec", "arena_parent_plans.sec"} {
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
					nested := 0
					epochs := map[InvalidationDomainID]map[EpochDependencyID]bool{}
					for call, stored := range a.arenaOperationPlans {
						plan, ok := a.ResolvedArenaOperationPlanOf(call)
						if !ok || !reflect.DeepEqual(plan, stored) {
							t.Fatal("plan snapshot mismatch")
						}
						if len(plan.Parents) == 0 {
							continue
						}
						if len(plan.Parents) != 1 {
							t.Fatal("fixture unexpectedly has joined parents")
						}
						parent := plan.Parents[0]
						if parent.Origin.Storage != StorageOriginArena || parent.BackingRelation != BackingRelationBorrowed || parent.ReclamationAuthority != ReclamationArena || parent.InvalidationDomain == 0 || parent.EpochDependency == 0 || parent.InvalidationDomain == plan.InvalidationDomain || plan.BackingKind != "borrowed" {
							t.Fatal(plan)
						}
						matched := false
						for _, allocation := range a.arenaOperationPlans {
							if allocation.Operation == ArenaOperationAlloc && allocation.InvalidationDomain == parent.InvalidationDomain && allocation.EpochAfter == parent.EpochDependency {
								matched = true
							}
						}
						if !matched {
							t.Fatal("parent snapshot disagrees with canonical allocating epoch")
						}
						if plan.Operation == ArenaOperationCreate {
							nested++
							if epochs[parent.InvalidationDomain] == nil {
								epochs[parent.InvalidationDomain] = map[EpochDependencyID]bool{}
							}
							epochs[parent.InvalidationDomain][parent.EpochDependency] = true
						}
						if plan.Operation == ArenaOperationRelease && (!plan.EndsDomain || !plan.ConsumesOwner || plan.EpochAfter != 0) {
							t.Fatal("child release lost its terminal contract")
						}
						plan.Parents[0].Origin.Name = "corrupt"
						plan.Parents[0].InvalidationDomain++
						again, _ := a.ResolvedArenaOperationPlanOf(call)
						if !reflect.DeepEqual(again, stored) {
							t.Fatal("nested parent query aliases stored data")
						}
					}
					expected := 7
					if file == "arena_parent_plans.sec" {
						expected = 2
					}
					if nested != expected {
						t.Fatalf("nested constructors=%d, want %d", nested, expected)
					}
					if file == "arena_parent_plans.sec" {
						if len(epochs) != 1 {
							t.Fatal("parent Reset changed parent identity")
						}
						for _, distinct := range epochs {
							if len(distinct) != 2 {
								t.Fatal("parent Reset did not distinguish retained epochs")
							}
						}
					}
					if before != fmt.Sprintf("%#v", a.placeIdentities) {
						t.Fatal("queries allocated identities")
					}
					previous := a.arenaOperationPlans
					a.Analyze(&ast.Program{})
					for call := range previous {
						if _, ok := a.ResolvedArenaOperationPlanOf(call); ok {
							t.Fatal("stale parent plan survived reanalysis")
						}
					}
				})
			}
		}
	}
}

// A joined backing retains every possible parent domain rather than selecting
// one scalar representative. Ending the child discharges both alternatives.
// Rules: rules/memory/arena.md — §51; rules/memory/lifetime_analysis.md — §11.1–11.3.
func TestArenaParentPlanJoinedOrigins(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_parent_join_plans.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	count := 0
	for call, stored := range a.arenaOperationPlans {
		if len(stored.Parents) == 0 {
			continue
		}
		plan, _ := a.ResolvedArenaOperationPlanOf(call)
		if len(plan.Parents) != 2 || plan.Parents[0].InvalidationDomain == plan.Parents[1].InvalidationDomain || plan.Parents[0].Origin.Name >= plan.Parents[1].Origin.Name {
			t.Fatal("joined backing collapsed or reordered parent alternatives", plan)
		}
		count++
	}
	if count != 3 {
		t.Fatalf("want child create/Reset/Release parent facts, got %d", count)
	}
}
