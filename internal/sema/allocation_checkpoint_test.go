package sema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAllocationCheckpoint verifies validated, dependency-bound persistence of
// free/allocating/unknown/mixed/recursive facts and conservative body coverage.
// Tampering with a decodable, rechecksummed summary still fails validation.
// Rules: rules/memory/allocation.md — §24(2),(4)-(6);
// rules/compiler/incremental_compilation.md — §§31-34,85.
func TestAllocationCheckpoint(t *testing.T) {
	path := "../../testdata/sema/allocation_facts_valid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	scope := CallGraphScope{"main", "linux-amd64", "allocation-model-v1"}
	deps := []CallGraphDependency{{"source", path, "source-v1"}, {"source-set", "main", "membership-v1"}}
	var index WorkspaceCallGraphIndex
	if errors, published, err := a.AnalyzeIndexed(program, &index, scope, deps); len(errors) != 0 || !published || err != nil {
		t.Fatal(errors, published, err)
	}
	if !reflect.DeepEqual(a.AllocationFacts(), a.CallGraph().AllocationFacts()) {
		t.Fatal("live and snapshot allocation facts disagree")
	}
	bound, err := a.CallGraph().ForCompilationPlan(scope)
	if err != nil {
		t.Fatal(err)
	}
	expected := bound.AllocationFacts()
	before, err := index.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	again, _ := index.Checkpoint()
	if !bytes.Equal(before, again) {
		t.Fatal("nondeterministic checkpoint")
	}
	current := map[CallGraphScope][]CallGraphDependency{scope: deps}
	restored, err := RestoreWorkspaceCallGraphIndex(before, current)
	if err != nil {
		t.Fatal(err)
	}
	graph, ok := restored.Snapshot(scope, deps)
	if !ok || !reflect.DeepEqual(expected, graph.AllocationFacts()) {
		t.Fatal("allocation facts changed on restore", expected, graph.AllocationFacts())
	}
	// Returned evidence and paths must not alias stored checkpoint facts.
	for _, fact := range graph.AllocationFacts() {
		if len(fact.UnknownEvidence) > 0 {
			fact.UnknownEvidence[0].Reason = "mutated"
		}
		if len(fact.AllocationPath) > 0 {
			fact.AllocationPath[0] = "mutated"
		}
	}
	if !reflect.DeepEqual(expected, graph.AllocationFacts()) {
		t.Fatal("mutable exported facts")
	}
	var envelope graphCheckpoint
	if err := json.Unmarshal(before, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(*persistedCallGraph){
		func(g *persistedCallGraph) { g.Allocation = nil },
		func(g *persistedCallGraph) { g.Allocation[0].Knowledge = "invented" },
		func(g *persistedCallGraph) { g.Allocation[0].Declaration.Line++ },
		func(g *persistedCallGraph) {
			for id, sites := range g.Arena {
				if len(sites) > 0 {
					sites[0].MayAllocate = !sites[0].MayAllocate
					g.Arena[id] = sites
					return
				}
			}
		},
		func(g *persistedCallGraph) {
			g.Coverage["missing-callable"] = []AllocationEvidence{{Reason: "invented"}}
		},
		func(g *persistedCallGraph) {
			for id, evidence := range g.Coverage {
				if len(evidence) > 0 {
					evidence[0].Reason = ""
					g.Coverage[id] = evidence
					return
				}
			}
		},
	} {
		var entries []graphCheckpointEntry
		if err := json.Unmarshal(envelope.Payload, &entries); err != nil {
			t.Fatal(err)
		}
		mutation(&entries[0].Graph)
		changed := envelope
		changed.Payload, _ = json.Marshal(entries)
		digest := sha256.Sum256(changed.Payload)
		changed.SHA256 = hex.EncodeToString(digest[:])
		wire, _ := json.Marshal(changed)
		if _, err := RestoreWorkspaceCallGraphIndex(wire, current); err == nil {
			t.Fatal("inconsistent allocation metadata accepted")
		}
	}
	changedDeps := append([]CallGraphDependency(nil), deps...)
	changedDeps[0].Fingerprint = "source-v2"
	stale, err := RestoreWorkspaceCallGraphIndex(before, map[CallGraphScope][]CallGraphDependency{scope: changedDeps})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stale.Snapshot(scope, changedDeps); ok {
		t.Fatal("stale summaries imported")
	}
	// Missing coverage on a manually built graph is unresolved, never free.
	for _, node := range graph.Nodes() {
		delete(graph.allocationCoverage, node.ID)
	}
	for _, fact := range graph.AllocationFacts() {
		if fact.Knowledge == AllocationFree {
			t.Fatal("missing coverage became positive freedom", fact)
		}
	}
}
