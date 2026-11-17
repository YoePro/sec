package sema

import (
	"os"
	"reflect"
	"testing"
)

// TestTrustProvenanceGraph checks known trust and unresolved implementation
// provenance independently through canonical direct, indirect, recursive and
// deferred execution edges. Trust does not change safe wrapper call contracts.
// Rules: rules/memory/unsafe.md §§3.4,13–14;
// rules/analysis/call_graph.md §§44–45, "One canonical graph, multiple analysis views".
func TestTrustProvenanceGraph(t *testing.T) {
	data, err := os.ReadFile("../../testdata/analysis/trust_graph.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, issues := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, issues, nil)
	graph := a.CallGraph()
	id := func(name string) CallableID { return callGraphNodeIDByName(t, graph, name) }
	for _, name := range []string{"Open", "OuterOpen", "RecursiveOpen", "DeferredOpen", "Mixed"} {
		t.Run(name, func(t *testing.T) {
			facts := a.TrustProvenance(id(name))
			if len(facts.UnknownPath) == 0 || facts.UnknownPath[0] != id(name) || facts.UnknownPath[len(facts.UnknownPath)-1] != id("Open") {
				t.Fatalf("lost open-target witness: %+v", facts)
			}
			if facts.CallerFacingUnsafe {
				t.Fatal("trust uncertainty changed the public safety contract")
			}
			if name != "Mixed" && facts.DependsOnTrust() {
				t.Fatal("unknown target was misclassified as known unsafe/foreign code")
			}
			if name == "Mixed" && !facts.DependsOnTrust() {
				t.Fatal("unknown target hid an independent known trust dependency")
			}
			if name == "DeferredOpen" && len(facts.UnknownPath) < 3 {
				t.Fatal("deferred body edge omitted")
			}
			if !reflect.DeepEqual(facts, a.TrustProvenance(id(name))) {
				t.Fatal("non-deterministic trust facts")
			}
		})
	}
	for _, name := range []string{"Plain", "Closed"} {
		facts := a.TrustProvenance(id(name))
		if facts.DependsOnTrust() || len(facts.UnknownPath) != 0 {
			t.Fatalf("invented trust for %s: %+v", name, facts)
		}
	}
	foreign := a.TrustProvenance(id("foreign_value"))
	if !foreign.ForeignDeclaration || !foreign.DependsOnTrust() {
		t.Fatalf("foreign implementation disappeared: %+v", foreign)
	}
	leaf := a.TrustProvenance(id("Leaf"))
	if !leaf.CallerFacingUnsafe || len(leaf.Direct) != 1 {
		t.Fatalf("lost introducing operation: %+v", leaf)
	}
	cycle := a.TrustProvenance(id("CycleB"))
	want := []CallableID{id("CycleB"), id("CycleA"), id("Wrapper")}
	if !reflect.DeepEqual(cycle.Path, want) {
		t.Fatalf("recursive witness = %v, want %v", cycle.Path, want)
	}

	// A consumer may change a returned snapshot without changing another
	// query, the operation registry, or either trust path in the analyzer.
	leaf.Direct[0].Obligations[0] = "changed"
	leaf.Direct[0].Operation = "changed"
	if next := a.TrustProvenance(id("Leaf")); next.Direct[0].Obligations[0] == "changed" || next.Direct[0].Operation == "changed" {
		t.Fatal("direct facts alias analyzer state")
	}
	cycle.Path[0] = "changed"
	if a.TrustProvenance(id("CycleB")).Path[0] != id("CycleB") {
		t.Fatal("known witness aliases analyzer state")
	}
	open := a.TrustProvenance(id("OuterOpen"))
	open.UnknownPath[0] = "changed"
	if a.TrustProvenance(id("OuterOpen")).UnknownPath[0] != id("OuterOpen") {
		t.Fatal("unknown witness aliases analyzer state")
	}
	callsForeign := a.TrustProvenance(id("Foreign"))
	if len(callsForeign.Foreign) != 1 || callsForeign.Foreign[0] != id("foreign_value") {
		t.Fatal("lost foreign edge")
	}
	callsForeign.Foreign[0] = "changed"
	if a.TrustProvenance(id("Foreign")).Foreign[0] != id("foreign_value") {
		t.Fatal("foreign facts alias analyzer state")
	}
}
