package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Every canonical operation preserves its object/order and path-sensitive
// modification/read/RMW classification. Facts express runtime constraints,
// never a guessed ordering across independent objects or execution contexts.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–17,24,52.
func TestAtomicMemoryModelOperations(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomic_memory_model_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(data), "atomic_memory_model_valid.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
			assertSemaErrors(t, a.Analyze(program), nil)
			plans := a.AtomicMemoryOperations()
			state := NewSynchronizationResourceSnapshot()
			if len(plans) != 22 {
				t.Fatalf("width %d depth %v: expected 22 operations, got %d: %+v", width, depth, len(plans), plans)
			}
			for i, plan := range plans {
				if plan.Operation == "Fence" || !plan.StorageResolved || !plan.TotalOrderPerObject || !plan.RequiresHappensBeforeCoherence || plan.Source.Line <= 0 || plan.Callable == "" {
					t.Fatalf("missing canonical storage/order facts: %+v", plan)
				}
				if i > 0 && plans[i-1].Source.ByteStart >= plan.Source.ByteStart {
					t.Fatal("source order/duplicate query unstable")
				}
				if AtomicModificationOrderRelationship(plan, plan, state) != ResourceSame {
					t.Fatalf("lost own modification domain: %+v", plan)
				}
				for j, path := range plan.Paths {
					wantRead := plan.Operation != "Store"
					wantWrite := plan.Operation != "Load" && !(plan.Operation == "CompareExchange" && j == 1)
					wantRMW := wantWrite && plan.Operation != "Store"
					if path.ReadsStorage != wantRead || path.ModifiesStorage != wantWrite || path.ParticipatesInModificationOrder != wantWrite || path.ReadModifyWrite != wantRMW || path.ReadsImmediatelyPrecedingModification != wantRMW {
						t.Fatalf("wrong memory classification for %s: %+v", plan.Operation, path)
					}
				}
				if plan.Operation == "CompareExchange" {
					if len(plan.Paths) != 2 || plan.Paths[0].Variant != "Exchanged" || plan.Paths[1].Variant != "NotExchanged" {
						t.Fatalf("CAS outcomes conflated: %+v", plan)
					}
				}
			}
			if plans[8].Paths[0].Order != "AcqRel" || plans[8].Paths[1].Order != "Acquire" {
				t.Fatal("independent CAS orders lost")
			}
			for _, path := range plans[9].Paths {
				if path.Order != "SeqCst" {
					t.Fatal("default CAS orders lost")
				}
			}
			if plans[18].Paths[0].Order != "Release" {
				t.Fatal("validated immutable order alias lost")
			}
			for _, plan := range plans[len(plans)-3:] {
				if plan.Paths[0].Order != "SeqCst" {
					t.Fatal("default order lost")
				}
			}
			// Parameters may alias; known repeated objects and field projections
			// retain identity without deriving it from equal identifier spellings.
			if AtomicModificationOrderRelationship(plans[10], plans[11], state) != ResourceMayAlias || AtomicModificationOrderRelationship(plans[10], plans[12], state) != ResourceSame {
				t.Fatalf("unresolved parameter identities collapsed: %s %s; roots: %+v %+v %+v", AtomicModificationOrderRelationship(plans[10], plans[11], state), AtomicModificationOrderRelationship(plans[10], plans[12], state), struct{ P Place }{plans[10].Storage.Place()}, struct{ P Place }{plans[11].Storage.Place()}, struct{ P Place }{plans[12].Storage.Place()})
			}
			if AtomicModificationOrderRelationship(plans[13], plans[14], state) != ResourceDisjoint || AtomicModificationOrderRelationship(plans[13], plans[15], state) != ResourceSame {
				t.Fatal("field modification orders merged")
			}
			if AtomicModificationOrderRelationship(plans[16], plans[17], state) != ResourceSame {
				t.Fatal("borrow alias split one atomic object")
			}
			snapshot := a.AtomicMemoryOperations()
			plans[0].Paths[0].Order = "changed"
			plans[0].Storage.place.Root = "changed"
			plans[0].Storage.place.Type.Name = "changed"
			if !reflect.DeepEqual(a.AtomicMemoryOperations(), snapshot) {
				t.Fatal("query exposes mutable analyzer-owned facts")
			}
			// Separate snapshots may reuse source and root numbers; that is not proof
			// of one shared storage object across analyses.
			b := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			assertSemaErrors(t, b.Analyze(program), nil)
			if AtomicModificationOrderRelationship(snapshot[0], b.AtomicMemoryOperations()[0], state) != ResourceUnknown {
				t.Fatal("independent snapshots share a modification domain")
			}
			var first *ast.CallExpression
			for call := range a.resolvedAtomicMemoryOperations {
				first = call
				break
			}
			assertSemaErrors(t, a.Analyze(parser.New(lexer.New("module main\nfn Clean() void {}\n")).ParseProgram()), nil)
			if len(a.AtomicMemoryOperations()) != 0 {
				t.Fatal("stale operations after reanalysis")
			}
			if _, ok := a.ResolvedAtomicMemoryOperationOf(first); ok {
				t.Fatal("stale query after reanalysis")
			}
		}
	}
}

// Rejected operations and unanalysed source never acquire successful memory
// facts through a read-only query; fences have no atomic-object modification.
// Rules: rules/concurrency/concurrency_memory_model.md §§16(5–8),52(5–7).
func TestAtomicMemoryModelInvalid(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomic_memory_model_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errs := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errs) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	if len(a.resolvedCompareExchanges) != 0 {
		t.Fatal("rejected CAS retained a successful path plan")
	}
	if len(a.AtomicMemoryOperations()) != 0 {
		t.Fatalf("invalid operation published ordering facts: %+v", a.AtomicMemoryOperations())
	}
	if _, ok := a.ResolvedAtomicMemoryOperationOf(&ast.CallExpression{}); ok {
		t.Fatal("query created facts")
	}
	var absent *Analyzer
	if len(absent.AtomicMemoryOperations()) != 0 {
		t.Fatal("nil query created facts")
	}
	if AtomicModificationOrderRelationship(ResolvedAtomicMemoryOperation{}, ResolvedAtomicMemoryOperation{}, nil) != ResourceUnknown {
		t.Fatal("unresolved storage invented identity")
	}
}

// Actual owned atomics, aliases and indexed subobjects retain four-state
// provenance precision. Source position is never used as an object identity.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–17,52(2).
func TestAtomicModificationOrderDomains(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomic_memory_domains_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errs := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errs, nil)
	plans := a.AtomicMemoryOperations()
	// This fixture has no move/replacement; each function is observed in one
	// source execution context. The owning analysis supplies that qualification.
	state := NewSynchronizationResourceSnapshot()
	if len(plans) != 7 {
		t.Fatalf("want seven domain observations, got %d", len(plans))
	}
	for _, tc := range []struct {
		left, right int
		want        SynchronizationResourceRelationship
	}{
		{0, 1, ResourceSame}, {0, 2, ResourceDisjoint}, {0, 3, ResourceSame},
		{4, 5, ResourceDisjoint}, {4, 6, ResourceMayAlias},
	} {
		if got := AtomicModificationOrderRelationship(plans[tc.left], plans[tc.right], state); got != tc.want {
			t.Fatalf("domains %d/%d: got %s want %s", tc.left, tc.right, got, tc.want)
		}
	}
	// Mutation of detached projected facts must not alter stored identity.
	plans[4].Storage.place.Projections[0].ConstantIndex.SetInt64(99)
	fresh := a.AtomicMemoryOperations()
	if AtomicModificationOrderRelationship(fresh[4], fresh[5], state) != ResourceDisjoint {
		t.Fatal("projection snapshot aliases stored state")
	}
}
