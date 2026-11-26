package deadlock

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
)

// Concurrent navigation requests read the same compact snapshot and receive
// independent expanded paths and uncertainty lists.
// Rules: rules/analysis/deadlock_analysis.md §§61,82.
func TestConcurrentCycleCauseDetails(t *testing.T) {
	g := causeFixture()
	g.Evidence["ab-2"] = g.Evidence["ab-1"]
	result, err := Analyze(g)
	if err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for round := 0; round < 10; round++ {
				details, err := result.CycleDetails(0)
				if err != nil || len(details) != 2 || len(details[0].Calls) != 2 || details[0].Calls[0].Caller != "Handler" {
					t.Error("invalid shared cause details", details, err)
					return
				}
				details[0].Calls[0].Caller = "local mutation"
				details[0].Evidence.Uncertainty[0] = "local mutation"
			}
		}()
	}
	readers.Wait()
}

func causeFixture() Graph {
	g := fixture(WaitForGraph)
	g.Causes = []CauseFrame{
		{ID: "entry", CallSite: "handler-update", Caller: "Handler", Callee: "Update", File: "handler.sec", Line: 7, Column: 4, Execution: "synchronous"},
		{ID: "store", Parent: "entry", CallSite: "update-store", Caller: "Update", Callee: "Cache.Store", File: "cache.sec", Line: 11, Column: 2, Execution: "deferred"},
		{ID: "log", Parent: "entry", CallSite: "update-log", Caller: "Update", Callee: "Log.Write", File: "log.sec", Line: 3, Column: 6},
		{ID: "unused", Parent: "entry", CallSite: "update-metrics", Caller: "Update", Callee: "Metrics.Record"},
		{ID: "unrelated", CallSite: "other-call", Caller: "Other", Callee: "Work"},
	}
	g.Evidence = map[EdgeID]EdgeEvidence{
		"ab-1": {Cause: "store", Uncertainty: []string{"path"}},
		"ba":   {Cause: "log"},
		"bc":   {Cause: "unused"},
	}
	return g
}

// Every selected edge retains its exact root-to-leaf call sequence. Common
// prefixes are stored once; unrelated and nonrepresentative paths are omitted
// without dropping original dependency or source evidence.
// Rules: rules/analysis/deadlock_analysis.md §§49,59,61,82.
func TestCycleCausePaths(t *testing.T) {
	g := causeFixture()
	// Keep the original deterministic ab-1 witness despite its uncertainty.
	v := g.Evidence["ab-2"]
	v.Uncertainty = []string{"path", "alias"}
	g.Evidence["ab-2"] = v
	got, err := Analyze(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Causes) != 3 {
		t.Fatal("unrelated paths leaked into compact result", got.Causes)
	}
	details, err := got.CycleDetails(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 2 || details[0].Edge != "ab-1" || !details[0].CauseAvailable || details[1].Edge != "ba" || !details[1].CauseAvailable {
		t.Fatal(details)
	}
	if !reflect.DeepEqual(details[0].Calls, []CauseFrame{g.Causes[0], g.Causes[1]}) || !reflect.DeepEqual(details[1].Calls, []CauseFrame{g.Causes[0], g.Causes[2]}) {
		t.Fatal("lost call identity, source, order or execution", details)
	}
	// Another cycle has no producer path, independently of graph cyclicity.
	missing, err := got.CycleDetails(3)
	if err != nil || len(missing) != 2 || missing[0].CauseAvailable || missing[1].CauseAvailable || len(missing[0].Calls) != 0 {
		t.Fatal(missing, err)
	}
	checkResult(t, g, got)
	// Detailed queries and result tables cannot mutate each other or input.
	details[0].Calls[0].Caller = "changed"
	details[0].Evidence.Uncertainty[0] = "changed"
	again, err := got.CycleDetails(0)
	if err != nil || again[0].Calls[0].Caller != "Handler" || again[0].Evidence.Uncertainty[0] != "path" || g.Causes[0].Caller != "Handler" {
		t.Fatal(again, err)
	}
	got.Causes[0].CallSite = "changed"
	if g.Causes[0].CallSite == "changed" {
		t.Fatal("result aliases producer frames")
	}
}

// Prefix and dependency insertion order never changes selected explanations.
// An edge whose path is absent stays absent rather than becoming a direct-call
// claim or fabricated source location.
// Rules: rules/analysis/deadlock_analysis.md §§49,61,82.
func TestCauseDeterminismAndMissingPaths(t *testing.T) {
	g := causeFixture()
	baseline, err := Analyze(g)
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewSource(61))
	for i := 0; i < 50; i++ {
		random.Shuffle(len(g.Causes), func(i, j int) { g.Causes[i], g.Causes[j] = g.Causes[j], g.Causes[i] })
		random.Shuffle(len(g.Edges), func(i, j int) { g.Edges[i], g.Edges[j] = g.Edges[j], g.Edges[i] })
		got, err := Analyze(g)
		if err != nil || !reflect.DeepEqual(got, baseline) {
			t.Fatal("cause order changed snapshot", got, err)
		}
	}
	g.Evidence = nil
	got, err := Analyze(g)
	if err != nil || len(got.Causes) != 0 {
		t.Fatal(got, err)
	}
	details, err := got.CycleDetails(0)
	if err != nil || len(details) != 2 || details[0].CauseAvailable || details[1].CauseAvailable {
		t.Fatal(details, err)
	}
	for _, index := range []int{-1, len(got.Components)} {
		if _, err := got.CycleDetails(index); err == nil {
			t.Fatal("invalid component accepted")
		}
	}
}

// Bad chains and references fail before publication; diagnostics cannot
// navigate a made-up intermediate call or a different parent callable.
// Rules: rules/analysis/deadlock_analysis.md §§49,61,82.
func TestMalformedCycleCauses(t *testing.T) {
	for _, change := range []func(*Graph){
		func(g *Graph) { g.Causes[0].ID = "" },
		func(g *Graph) { g.Causes = append(g.Causes, g.Causes[0]) },
		func(g *Graph) { g.Causes[0].Caller = "" },
		func(g *Graph) { g.Causes[0].Callee = "" },
		func(g *Graph) { g.Causes[0].CallSite = "" },
		func(g *Graph) { g.Causes[0].Line = -1 },
		func(g *Graph) { g.Causes[1].Parent = "absent" },
		func(g *Graph) { g.Causes[1].Caller = "DifferentCallable" },
		func(g *Graph) { g.Causes[0].Parent = "entry"; g.Causes[0].Caller = g.Causes[0].Callee },
		func(g *Graph) { v := g.Evidence["ab-1"]; v.Cause = "missing"; g.Evidence["ab-1"] = v },
	} {
		g := causeFixture()
		change(&g)
		if got, err := Analyze(g); err == nil || !reflect.DeepEqual(got, Result{}) {
			t.Fatal("malformed cause accepted or partly published", got, err)
		}
	}
	g := causeFixture()
	got, err := Analyze(g)
	if err != nil {
		t.Fatal(err)
	}
	got.Causes[0].Parent = "missing"
	if details, err := got.CycleDetails(0); err == nil || details != nil {
		t.Fatal("malformed result table expanded", details, err)
	}
}

// Finite recursive/reentrant paths may repeat a callable and call-site identity
// while their prefix IDs remain distinct. Deep paths use no Go recursion and
// common prefixes are not copied once for every cycle edge in normal results.
// Rules: rules/analysis/deadlock_analysis.md §§44-45,59,61,82.
func TestDeepSharedRecursiveCausePath(t *testing.T) {
	const depth = 30000
	g := Graph{Kind: WaitForGraph, Scope: "recursive-path", Nodes: []NodeID{"a", "b"}, Edges: []Edge{{"ab", "a", "b"}, {"ba", "b", "a"}}, Evidence: map[EdgeID]EdgeEvidence{}}
	var previous CauseID
	for i := depth; i > 0; i-- {
		id := CauseID(fmt.Sprintf("prefix%05d", i))
		g.Causes = append(g.Causes, CauseFrame{ID: id, Parent: previous, CallSite: "reentry", Caller: "Recursive", Callee: "Recursive"})
		previous = id
	}
	g.Evidence["ab"] = EdgeEvidence{Cause: previous}
	g.Evidence["ba"] = EdgeEvidence{Cause: previous}
	got, err := Analyze(g)
	if err != nil || len(got.Causes) != depth {
		t.Fatal("deep compact cause graph", len(got.Causes), err)
	}
	details, err := got.CycleDetails(0)
	if err != nil || len(details) != 2 || len(details[0].Calls) != depth || len(details[1].Calls) != depth {
		t.Fatal("deep path expansion", err)
	}
	if details[0].Calls[0].ID != "prefix30000" || details[0].Calls[depth-1].ID != "prefix00001" {
		t.Fatal("wrong cause order")
	}
}
