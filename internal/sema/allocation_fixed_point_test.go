package sema

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAllocationRecursiveFixedPoint checks complete recursive components at
// every analysis depth, with independent positive/unknown propagation, defer,
// trusted foreign boundaries and spawned-body exclusion. Shortest witnesses
// are compared with independent graph reachability, including source-order ties.
// Rules: rules/memory/allocation.md — §§6(3)-(4),24,29;
// rules/analysis/effect_analysis.md — "Fixed-point analysis";
// rules/analysis/call_graph.md — "Same-stack effect transfer", "Spawn effect transfer", A.21.
func TestAllocationRecursiveFixedPoint(t *testing.T) {
	file := "../../testdata/sema/allocation_recursive_fixed_point_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]AllocationKnowledge{
		"Foreign": AllocationUnknown, "Trusted": AllocationFree, "Allocate": AllocationMayAllocate,
		"FreeA": AllocationFree, "FreeB": AllocationFree, "FreeRoot": AllocationFree, "main": AllocationFree,
		"AllocA": AllocationMayAllocate, "AllocB": AllocationMayAllocate, "AllocC": AllocationMayAllocate,
		"UnknownA": AllocationUnknown, "UnknownB": AllocationUnknown,
		"MixedA": AllocationMayAllocate, "MixedB": AllocationMayAllocate,
		"DeferredA": AllocationMayAllocate, "DeferredB": AllocationMayAllocate, "defer": AllocationMayAllocate,
		"TrustedA": AllocationFree, "TrustedB": AllocationFree,
		"SpawnA": AllocationUnknown, "SpawnB": AllocationUnknown,
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			analyzer := NewAnalyzerWithDepth(depth)
			if errors := analyzer.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			graph := analyzer.CallGraph()
			for _, node := range graph.Nodes() {
				want, found := expected[node.Name]
				if !found {
					t.Fatal("unexpected node", node.Name)
				}
				fact := analyzer.AllocationFact(node.ID)
				if fact.Knowledge != want {
					t.Fatalf("%s: %+v; want %s", node.Name, fact, want)
				}
				summary := graph.ArenaSummary(node.ID)
				if (node.Name == "MixedA" || node.Name == "MixedB") && (!summary.MayAllocate || !summary.AllocationUnknown) {
					t.Fatal("lost independent effect", summary)
				}
				if (node.Name == "SpawnA" || node.Name == "SpawnB") && summary.MayAllocate {
					t.Fatal("spawn body leaked", summary)
				}
				for _, unknown := range []bool{false, true} {
					expectedPath := graph.synchronousPathTo(node.ID, func(id CallableID) bool {
						for _, effect := range graph.arenaEffects[id] {
							if !unknown && effect.MayAllocate || unknown && effect.UnknownAllocation {
								return true
							}
						}
						return false
					})
					actualPath := summary.AllocationPath
					if unknown {
						actualPath = summary.UnknownAllocationPath
					}
					if !reflect.DeepEqual(actualPath, expectedPath) {
						t.Fatalf("%s unknown=%v: got %v, want %v", node.Name, unknown, actualPath, expectedPath)
					}
				}
			}
		})
	}
}

// TestAllocationFixedPointLongCycle checks convergence beyond interactive
// budgets, empty cycles, new facts/edges after earlier queries, detached
// snapshots and concurrent immutable-view readers.
// Rules: rules/analysis/effect_analysis.md — "Fixed-point analysis";
// rules/memory/allocation.md — §§24(4),(6),29(5).
func TestAllocationFixedPointLongCycle(t *testing.T) {
	const count = 256
	graph := newCallGraph()
	functions := make([]Function, count)
	ids := make([]CallableID, count)
	for i := range functions {
		functions[i] = Function{Name: fmt.Sprintf("Node%d", i), Token: lexer.Token{File: "cycle.sec", Line: i + 1, Column: 1}}
		ids[i] = graph.addCallable(functions[i])
	}
	for i := range functions {
		graph.addCall(ids[i], functions[(i+1)%count], functions[i].Token, CallDispatchDirect, CallExecutionSynchronous)
	}
	before := graph.clone()
	for _, id := range ids {
		if result := graph.ArenaSummary(id); result.MayAllocate || result.AllocationUnknown {
			t.Fatal("empty cycle acquired an effect", result)
		}
	}
	graph.addArenaEffect(ids[count-1], ArenaEffectSite{Kind: ArenaEffectAllocate, Source: functions[count-1].Token, MayAllocate: true})
	graph.addArenaEffect(ids[count/2], ArenaEffectSite{Kind: ArenaEffectForeign, Source: functions[count/2].Token, UnknownAllocation: true})
	root := graph.ArenaSummary(ids[0])
	if !root.MayAllocate || !root.AllocationUnknown || len(root.AllocationPath) != count || len(root.UnknownAllocationPath) != count/2+1 {
		t.Fatal("long cycle did not converge", root)
	}
	root.AllocationPath[0] = "mutated"
	root.DirectEffects = append(root.DirectEffects, ArenaEffectSite{MayAllocate: true})
	if got := graph.ArenaSummary(ids[0]); got.AllocationPath[0] != ids[0] || len(got.DirectEffects) != 0 {
		t.Fatal("returned facts alias graph", got)
	}
	graph.addCall(ids[0], functions[count-1], lexer.Token{File: "cycle.sec", Line: 1, Column: 2}, CallDispatchDirect, CallExecutionSynchronous)
	if got := graph.ArenaSummary(ids[0]); !reflect.DeepEqual(got.AllocationPath, []CallableID{ids[0], ids[count-1]}) {
		t.Fatal("new edge did not replace shortest witness", got)
	}
	if got := before.ArenaSummary(ids[0]); got.MayAllocate || got.AllocationUnknown {
		t.Fatal("historical clone changed", got)
	}
	snapshot := graph.clone()
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for _, id := range ids {
				got := snapshot.ArenaSummary(id)
				if !got.MayAllocate || !got.AllocationUnknown {
					t.Errorf("missing recursive summary for %s", id)
				}
			}
		}()
	}
	readers.Wait()
}

// TestAllocationRecursivePolicy preserves mandatory noalloc errors and their
// detached positive/unknown witnesses across recursive components at every depth.
// Rules: rules/memory/allocation.md — §§24(6),28(4),29(4).
func TestAllocationRecursivePolicy(t *testing.T) {
	file := "../../testdata/sema/allocation_recursive_fixed_point_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		errors := NewAnalyzerWithDepth(depth).Analyze(program)
		if len(errors) != 2 {
			t.Fatalf("%s: %v", depth, errors)
		}
		unknown := 0
		for _, diagnostic := range errors {
			if diagnostic.ID != "S1108" || diagnostic.AllocationCause == nil || len(diagnostic.AllocationCause.Steps) < 3 {
				t.Fatal(diagnostic)
			}
			if diagnostic.AllocationCause.Unknown {
				unknown++
			}
		}
		if unknown != 1 {
			t.Fatal("lost independent unknown witness", errors)
		}
	}
}
