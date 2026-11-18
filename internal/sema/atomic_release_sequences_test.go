package sema

import (
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Release heads and chain transfer are outcome-sensitive symbolic constraints.
// NoModification outcomes are transparent, stores break all prior heads, and
// release RMWs retain inherited heads as well as their own head, even when the
// caller or order differs. Source adjacency alone never proves membership.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–20,25–28,52.
func TestAtomicReleaseSequenceFacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomic_release_sequences.sec")
	if err != nil {
		t.Fatal(err)
	}
	// H: release head, C: continues predecessor heads, B: breaks prior
	// heads, N: no modification. RMWs may both H and C; stores may H and B.
	want := [][]string{{"HB"}, {"C"}, {"C", "N"}, {"HC"}, {"C"}, {"B"}, {"HC"}, {"N"}, {"HB"}, {"HC", "N"}, {"C"}, {"HC"}, {"HB"}, {"C"}}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(data), "atomic_release_sequences.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
			assertSemaErrors(t, a.Analyze(program), nil)
			plans := a.AtomicMemoryOperations()
			if len(plans) != len(want) {
				t.Fatalf("plans: %+v", plans)
			}
			for i, plan := range plans {
				if len(plan.Paths) != len(want[i]) {
					t.Fatalf("paths: %+v", plan)
				}
				for j, path := range plan.Paths {
					effect := path.ReleaseSequence
					role := ""
					if effect.StartsHead {
						role += "H"
					}
					if effect.ContinuesPredecessorHeads {
						role += "C"
					}
					if effect.BreaksPredecessorHeads {
						role += "B"
					}
					if effect.NoModification {
						role += "N"
					}
					if role != want[i][j] {
						t.Fatalf("%d/%s/%s: %+v, want %s", i, plan.Operation, path.Order, effect, want[i][j])
					}
					if effect.RequiresSameObject != path.ModifiesStorage || effect.RequiresContiguousRMW != path.ReadModifyWrite {
						t.Fatalf("lost chain constraints: %+v", path)
					}
					if path.Variant == "NotExchanged" && (effect.StartsHead || effect.ContinuesPredecessorHeads || effect.BreaksPredecessorHeads) {
						t.Fatal("failed CAS changed modification chain")
					}
				}
			}
			if plans[10].Callable == plans[0].Callable {
				t.Fatal("missing independent execution context")
			}
			// A different unresolved reference can alias the head storage:
			// it cannot establish a proven release sequence merely by order.
			state := NewSynchronizationResourceSnapshot()
			if AtomicModificationOrderRelationship(plans[12], plans[13], state) != ResourceMayAlias {
				t.Fatal("unproven chain gained object identity")
			}
			if AtomicModificationOrderRelationship(plans[0], plans[1], nil) != ResourceUnknown {
				t.Fatal("unqualified chain gained a common state")
			}
			plans[0].Paths[0].ReleaseSequence.StartsHead = false
			if !a.AtomicMemoryOperations()[0].Paths[0].ReleaseSequence.StartsHead {
				t.Fatal("query mutated canonical release facts")
			}
			clean := parser.New(lexer.New("module main\nfn Clean() void {}\n")).ParseProgram()
			assertSemaErrors(t, a.Analyze(clean), nil)
			if len(a.AtomicMemoryOperations()) != 0 {
				t.Fatal("stale release facts after reanalysis")
			}
		}
	}
}

// The effect algebra preserves overlapping heads and exactly implements
// §27's Store break. This is a test of the emitted transfer constraints on a
// hypothetical proven per-object modification path, not a source-order proof.
// Rules: rules/concurrency/concurrency_memory_model.md §§25–28.
func TestAtomicReleaseSequenceTransferConstraints(t *testing.T) {
	headStore := atomicReleaseSequenceEffect(AtomicMemoryPath{Order: "Release", ModifiesStorage: true})
	relaxedRMW := atomicReleaseSequenceEffect(AtomicMemoryPath{Order: "Relaxed", ModifiesStorage: true, ReadModifyWrite: true})
	releaseRMW := atomicReleaseSequenceEffect(AtomicMemoryPath{Order: "Release", ModifiesStorage: true, ReadModifyWrite: true})
	failedCAS := atomicReleaseSequenceEffect(AtomicMemoryPath{Order: "SeqCst", ReadsStorage: true})
	plainStore := atomicReleaseSequenceEffect(AtomicMemoryPath{Order: "Relaxed", ModifiesStorage: true})
	heads := map[int]bool{}
	steps := []AtomicReleaseSequenceEffect{headStore, relaxedRMW, failedCAS, releaseRMW, relaxedRMW, plainStore, relaxedRMW, headStore}
	expected := [][]int{{0}, {0}, {0}, {0, 3}, {0, 3}, {}, {}, {7}}
	for i, effect := range steps {
		if effect.BreaksPredecessorHeads {
			clear(heads)
		}
		if effect.StartsHead {
			heads[i] = true
		}
		if len(heads) != len(expected[i]) {
			t.Fatalf("step %d: heads %v, want %v", i, heads, expected[i])
		}
		for _, head := range expected[i] {
			if !heads[head] {
				t.Fatalf("lost overlapping head %d at %d", head, i)
			}
		}
	}
}
