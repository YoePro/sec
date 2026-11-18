package sema

import (
	"reflect"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestChildPanicDomains keeps synchronous argument effects in the parent while
// task/thread bodies, lambdas, closed function values and foreign targets get
// separate child-domain witnesses without assumed runtime containment.
// Rules: rules/errors/panic.md §§6–9,21,28(7); tasks.md §14; threads.md §56.
func TestChildPanicDomains(t *testing.T) {
	a, program, errors := panicCauseAnalyzer(t, "child_panic_domains_valid")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	g := a.CallGraph()
	for _, name := range []string{"SpawnTask", "SpawnThread", "SpawnLambda", "SpawnThreadLambda", "SpawnValue", "SpawnGeneric", "SpawnCaptured", "SpawnForeign", "Middle", "Top"} {
		id := callGraphNodeIDByName(t, g, name)
		if g.EffectSummary(id).MayPanic {
			t.Fatalf("child panic taints parent %s: %+v", name, g.EffectSummary(id))
		}
		domains := g.ChildPanicDomains(id)
		if len(domains) != 1 || !domains[0].MayPanic {
			t.Fatalf("%s: %+v", name, domains)
		}
		if domains[0].Unknown != (name == "SpawnForeign") {
			t.Fatalf("%s: %+v", name, domains)
		}
		if name == "Top" && len(domains[0].ParentPath) != 2 {
			t.Fatal(domains)
		}
		if name != "SpawnForeign" && (len(domains[0].Causes) == 0 || domains[0].Causes[0].Effect == nil || !domains[0].Causes[0].Effect.HasMessage) {
			t.Fatal(name, domains)
		}
	}
	for _, name := range []string{"Synchronous", "ParentArgument"} {
		if !g.EffectSummary(callGraphNodeIDByName(t, g, name)).MayPanic {
			t.Fatal("lost parent panic", name)
		}
	}
	argument := g.ChildPanicDomains(callGraphNodeIDByName(t, g, "ParentArgument"))
	if len(argument) != 1 || argument[0].MayPanic || argument[0].Unknown {
		t.Fatal("argument effect moved to child", argument)
	}
	root := callGraphNodeIDByName(t, g, "Top")
	saved := g.ChildPanicDomains(root)
	changed := g.ChildPanicDomains(root)
	changed[0].Boundary.Targets[0] = "changed"
	changed[0].Boundary.TargetSet.KnownTargets[0] = "changed"
	changed[0].ParentPath[0].Targets[0] = "changed"
	changed[0].Causes[0].Effect.Message = "changed"
	if !reflect.DeepEqual(saved, g.ChildPanicDomains(root)) {
		t.Fatal("mutable child panic snapshot")
	}
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if !reflect.DeepEqual(saved, a.CallGraph().ChildPanicDomains(root)) {
		t.Fatal("nondeterministic repeated analysis")
	}
	_, _, errors = panicCauseAnalyzer(t, "child_panic_domains_invalid")
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if e.ID != diagnostics.NoPanicViolation {
			t.Fatal(e)
		}
	}
}

// TestChildPanicUnknownAndProcess uses canonical producer facts for open targets
// and process boundaries before source process spawning is implemented.
// Rules: rules/analysis/call_graph.md — "Conservative unknown facts", "Spawn effect transfer";
// rules/errors/panic.md §§6–9,21(4).
func TestChildPanicUnknownAndProcess(t *testing.T) {
	g := newCallGraph()
	root := Function{Name: "Root", Module: "main", Token: lexer.Token{Line: 1, Column: 1}}
	child := Function{Name: "Child", Module: "main", Token: lexer.Token{Line: 2, Column: 1}}
	id := g.addCallable(root)
	childID := g.addCallable(child)
	g.addEffect(childID, EffectSite{Kind: EffectMayPanicExplicit, Source: lexer.Token{Line: 3, Column: 1}})
	g.addCall(id, child, lexer.Token{Line: 4, Column: 1}, CallDispatchDirect, CallExecutionSpawnProcess)
	g.addTargetSetCall(id, CallableTargetSet{}, lexer.Token{Line: 5, Column: 1}, CallDispatchFunctionValue, CallExecutionSpawnTask)
	domains := g.ChildPanicDomains(id)
	if len(domains) != 2 || !domains[0].MayPanic || domains[0].Unknown || !domains[1].Unknown || !domains[1].MayPanic || g.EffectSummary(id).MayPanic {
		t.Fatal(domains)
	}
	// The same graph query must terminate through parent recursion and retain
	// one shortest boundary witness.
	g.addCall(id, root, lexer.Token{Line: 6, Column: 1}, CallDispatchDirect, CallExecutionSynchronous)
	if len(g.ChildPanicDomains(id)) != 2 {
		t.Fatal("recursive traversal changed child boundaries")
	}
}

// TestChildPanicDomainsCheckpoint derives the same domain witnesses from the
// canonical versioned workspace graph and rejects mismatched source freshness.
// Rules: rules/compiler/incremental_compilation.md §§31–32;
// rules/compiler/semantic_ir.md §109(4); panic.md §28(7).
func TestChildPanicDomainsCheckpoint(t *testing.T) {
	a, _, errors := panicCauseAnalyzer(t, "child_panic_domains_valid")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	scope := CallGraphScope{"child-panic", "linux-amd64", "sources-v1"}
	deps := []CallGraphDependency{{"source", "../../testdata/sema/child_panic_domains_valid.sec", "v1"}}
	var index WorkspaceCallGraphIndex
	lease, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(lease, a.CallGraph()); err != nil || !ok {
		t.Fatal(ok, err)
	}
	before, ok := index.Snapshot(scope, deps)
	if !ok {
		t.Fatal("missing snapshot")
	}
	data, err := index.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreWorkspaceCallGraphIndex(data, map[CallGraphScope][]CallGraphDependency{scope: deps})
	if err != nil {
		t.Fatal(err)
	}
	after, ok := restored.Snapshot(scope, deps)
	if !ok {
		t.Fatal("missing restored graph")
	}
	for _, name := range []string{"SpawnLambda", "SpawnCaptured", "SpawnGeneric", "SpawnForeign", "Top"} {
		first := before.ChildPanicDomains(callGraphNodeIDByName(t, before, name))
		second := after.ChildPanicDomains(callGraphNodeIDByName(t, after, name))
		if !reflect.DeepEqual(first, second) {
			t.Fatal("lost child panic checkpoint metadata", name)
		}
	}
	stale := []CallGraphDependency{{"source", "../../testdata/sema/child_panic_domains_valid.sec", "v2"}}
	if _, ok := restored.Snapshot(scope, stale); ok {
		t.Fatal("stale child panic facts reused")
	}
}
