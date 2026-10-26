package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"
)

// The canonical graph alone owns recursion: direct/mutual/indirect source
// calls and represented callback/foreign-reentry edges are recursive, while a
// spawned-worker relationship must not create same-stack recursion. Reentry
// facts here represent supplied graph metadata, not a new FFI source contract.
// Rules: rules/analysis/stack_analysis.md — "Call graph ownership", "Recursion and SCCs", "Unknown recursion depth", "Physical stack domains";
// rules/analysis/call_graph.md — "Same-stack execution", Appendix A.20.
func TestCanonicalStackComponentsAndConsumers(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/stack_canonical_sccs.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, nil)
	graph := analyzer.CallGraph()
	ids := map[string]CallableID{}
	for _, node := range graph.Nodes() {
		ids[node.Name] = node.ID
	}
	// Canonical producers can add represented callback/reentry relationships.
	for _, edge := range []struct {
		from, to string
		dispatch CallDispatchKind
	}{
		{"CallbackTarget", "CallbackHost", CallDispatchClosure},
		{"Foreign", "ForeignHost", CallDispatchForeign},
	} {
		node, _ := graph.Node(ids[edge.from])
		graph.sites = append(graph.sites, CallSite{ID: CallSiteID("represented-" + edge.from), Caller: ids[edge.from], Targets: []CallableID{ids[edge.to]}, Dispatch: edge.dispatch, Execution: CallExecutionSynchronous, Source: node.Declaration})
	}
	for index := range graph.sites {
		site := &graph.sites[index]
		if site.Caller == ids["SpawnHost"] {
			site.Execution = CallExecutionSpawnThread
		}
	}
	components := graph.SameStackComponents()
	seen := map[CallableID]bool{}
	for _, component := range components {
		for _, id := range component.Members {
			if seen[id] {
				t.Fatal("duplicate graph component membership")
			}
			seen[id] = true
			want := id == ids["Direct"] || id == ids["Left"] || id == ids["Right"] || id == ids["Indirect"] || id == ids["CallbackHost"] || id == ids["CallbackTarget"] || id == ids["Foreign"] || id == ids["ForeignHost"]
			wantMembers := 1
			if id == ids["Left"] || id == ids["Right"] || id == ids["CallbackHost"] || id == ids["CallbackTarget"] || id == ids["Foreign"] || id == ids["ForeignHost"] {
				wantMembers = 2
			}
			if len(component.Members) != wantMembers {
				t.Fatalf("wrong canonical component size for %s: %v", id, component.Members)
			}
			if component.Recursive != want || graph.IsSameStackRecursive(id) != want {
				t.Fatalf("recursion ownership mismatch for %s: %+v", id, component)
			}
			if len(graph.SameStackSCC(id)) != len(component.Members) {
				t.Fatal("single/batch component queries disagree")
			}
		}
	}
	if len(seen) != len(graph.Nodes()) {
		t.Fatal("canonical partition dropped nodes")
	}
	var frames StackSummaryStore
	frame, _ := NewExactStackBound(big.NewInt(64))
	for _, node := range graph.Nodes() {
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: frame}); err != nil {
			t.Fatal(err)
		}
		if err := frames.RecordMachine(MachineStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: frame}); err != nil {
			t.Fatal(err)
		}
	}
	semantic := ComposeSemanticStackSummaries(graph, &frames, "plan")
	machine, err := ComposeMachineStackSummaries(graph, &frames, "plan")
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range semantic {
		recursive := graph.IsSameStackRecursive(summary.Callable)
		if recursive && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("recursive component acquired a finite bound without a depth proof", summary)
		}
		if summary.Callable == ids["Leaf"] {
			bytes, finite := summary.TransitiveMaximum.Bytes()
			if !finite || bytes.Cmp(big.NewInt(64)) != 0 {
				t.Fatal("isolated leaf gained a false recursion boundary", summary)
			}
		}
	}
	for _, summary := range machine {
		if graph.IsSameStackRecursive(summary.Callable) && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("machine bound ignored canonical recursion", summary)
		}
	}
	reversed := graph.clone()
	for left, right := 0, len(reversed.nodeOrder)-1; left < right; left, right = left+1, right-1 {
		reversed.nodeOrder[left], reversed.nodeOrder[right] = reversed.nodeOrder[right], reversed.nodeOrder[left]
	}
	for left, right := 0, len(reversed.sites)-1; left < right; left, right = left+1, right-1 {
		reversed.sites[left], reversed.sites[right] = reversed.sites[right], reversed.sites[left]
	}
	if !reflect.DeepEqual(components, reversed.SameStackComponents()) {
		t.Fatal("component partition depends on graph registration order")
	}
	if !reflect.DeepEqual(semantic, ComposeSemanticStackSummaries(reversed, &frames, "plan")) {
		t.Fatal("semantic stack results depend on graph order")
	}
	reversedMachine, err := ComposeMachineStackSummaries(reversed, &frames, "plan")
	if err != nil || !reflect.DeepEqual(machine, reversedMachine) {
		t.Fatal("machine stack results depend on graph order", err)
	}
	components[0].Members[0] = "changed"
	components[0].Recursive = !components[0].Recursive
	if reflect.DeepEqual(components, graph.SameStackComponents()) {
		t.Fatal("component query exposed mutable graph state")
	}
	if (*CallGraph)(nil).SameStackComponents() != nil {
		t.Fatal("nil graph has components")
	}
}
