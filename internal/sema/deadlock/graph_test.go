package deadlock

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
)

func fixture(kind GraphKind) Graph {
	return Graph{Kind: kind, Scope: "producer-snapshot", Nodes: []NodeID{"alone", "a", "b", "c", "d", "e", "self"}, Edges: []Edge{
		{"ab-2", "a", "b"}, {"ab-1", "a", "b"}, {"ba", "b", "a"},
		{"bc", "b", "c"}, {"ac", "a", "c"}, {"ad", "a", "d"}, {"cd", "c", "d"},
		{"de", "d", "e"}, {"ed", "e", "d"}, {"self", "self", "self"},
	}}
}

// Parallel dependency evidence is retained, singleton self-waits are cycles,
// isolated nodes are not cycles, and condensation edges form an exact DAG.
// These are structural candidates, not deadlock classifications.
// Rules: rules/analysis/deadlock_analysis.md §§7-8,45,48-50,82.
func TestDependencyComponents(t *testing.T) {
	for _, kind := range []GraphKind{LockOrderGraph, WaitForGraph} {
		t.Run(string(kind), func(t *testing.T) {
			got, err := Analyze(fixture(kind))
			if err != nil {
				t.Fatal(err)
			}
			want := []Component{
				{Members: []NodeID{"a", "b"}, InternalEdges: []EdgeID{"ab-1", "ab-2", "ba"}, Cycle: []EdgeID{"ab-1", "ba"}},
				{Members: []NodeID{"alone"}},
				{Members: []NodeID{"c"}},
				{Members: []NodeID{"d", "e"}, InternalEdges: []EdgeID{"de", "ed"}, Cycle: []EdgeID{"de", "ed"}},
				{Members: []NodeID{"self"}, InternalEdges: []EdgeID{"self"}, Cycle: []EdgeID{"self"}},
			}
			arcs := []Arc{{From: 0, To: 2, Edges: []EdgeID{"ac", "bc"}}, {From: 0, To: 3, Edges: []EdgeID{"ad"}}, {From: 2, To: 3, Edges: []EdgeID{"cd"}}}
			if got.Kind != kind || got.Scope != "producer-snapshot" || !reflect.DeepEqual(got.Components, want) || !reflect.DeepEqual(got.Condensation, arcs) {
				t.Fatalf("%+v", got)
			}
			checkResult(t, fixture(kind), got)
		})
	}
}

// Reordering, concurrent readers and mutations of a returned snapshot cannot
// change the structural answer or alter input provenance.
// Rules: rules/analysis/deadlock_analysis.md §§48-50,82.
func TestDependencyDeterminismAndIsolation(t *testing.T) {
	graph := fixture(WaitForGraph)
	baseline, err := Analyze(graph)
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewSource(41))
	for i := 0; i < 100; i++ {
		random.Shuffle(len(graph.Nodes), func(i, j int) { graph.Nodes[i], graph.Nodes[j] = graph.Nodes[j], graph.Nodes[i] })
		random.Shuffle(len(graph.Edges), func(i, j int) { graph.Edges[i], graph.Edges[j] = graph.Edges[j], graph.Edges[i] })
		got, err := Analyze(graph)
		if err != nil || !reflect.DeepEqual(got, baseline) {
			t.Fatal("registration order changed result", got, err)
		}
	}
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			got, err := Analyze(graph)
			if err != nil || !reflect.DeepEqual(got, baseline) {
				t.Error(got, err)
			}
		}()
	}
	readers.Wait()
	got, _ := Analyze(graph)
	got.Edges[0].From = "changed"
	got.Components[0].Members[0] = "changed"
	got.Components[0].Cycle[0] = "changed"
	got.Components[0].InternalEdges[0] = "changed"
	got.Condensation[0].Edges[0] = "changed"
	again, _ := Analyze(graph)
	if !reflect.DeepEqual(again, baseline) {
		t.Fatal("published result aliases input", again)
	}
}

// Removing one necessary edge must remove its cycle immediately, even though
// other supporting edges and another independent cycle remain.
// Rules: rules/analysis/deadlock_analysis.md §§8,45,82.
func TestDependencyEdgeRemoval(t *testing.T) {
	graph := fixture(WaitForGraph)
	for i, edge := range graph.Edges {
		if edge.ID == "ba" {
			graph.Edges = append(graph.Edges[:i], graph.Edges[i+1:]...)
			break
		}
	}
	got, err := Analyze(graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, component := range got.Components {
		for _, node := range component.Members {
			if (node == "a" || node == "b") && len(component.Cycle) != 0 {
				t.Fatal("stale cycle", component)
			}
		}
	}
	checkResult(t, graph, got)
}

// Invalid snapshots fail atomically; unknown names are never synthesized into
// canonical resource/execution identities or graph coverage proofs.
// Rules: rules/analysis/deadlock_analysis.md §§7-8,47-50.
func TestInvalidDependencyGraphs(t *testing.T) {
	for _, change := range []func(*Graph){
		func(g *Graph) { g.Kind = "call-graph" }, func(g *Graph) { g.Scope = "" },
		func(g *Graph) { g.Nodes = append(g.Nodes, "") }, func(g *Graph) { g.Nodes = append(g.Nodes, "a") },
		func(g *Graph) { g.Edges[0].ID = "" }, func(g *Graph) { g.Edges = append(g.Edges, g.Edges[0]) },
		func(g *Graph) { g.Edges[0].From = "absent" }, func(g *Graph) { g.Edges[0].To = "absent" },
	} {
		graph := fixture(LockOrderGraph)
		change(&graph)
		got, err := Analyze(graph)
		if err == nil || !reflect.DeepEqual(got, Result{}) {
			t.Fatal("invalid graph accepted or partly published", got, err)
		}
	}
	got, err := Analyze(Graph{Kind: WaitForGraph, Scope: "empty"})
	if err != nil || len(got.Components) != 0 || len(got.Condensation) != 0 {
		t.Fatal(got, err)
	}
}

// An independent all-pairs reachability oracle checks the SCC partition on
// generated small graphs; every witness closes and condensation is acyclic.
// Rules: rules/analysis/deadlock_analysis.md §§45,58,82.
func TestDependencyReachabilityOracle(t *testing.T) {
	random := rand.New(rand.NewSource(82))
	for trial := 0; trial < 200; trial++ {
		count := 1 + random.Intn(10)
		graph := Graph{Kind: WaitForGraph, Scope: "oracle"}
		for i := 0; i < count; i++ {
			graph.Nodes = append(graph.Nodes, NodeID(fmt.Sprint(i)))
		}
		for i := 0; i < count; i++ {
			for j := 0; j < count; j++ {
				if random.Intn(4) == 0 {
					graph.Edges = append(graph.Edges, Edge{EdgeID(fmt.Sprintf("%d/%d", i, j)), graph.Nodes[i], graph.Nodes[j]})
				}
			}
		}
		got, err := Analyze(graph)
		if err != nil {
			t.Fatal(err)
		}
		checkResult(t, graph, got)
	}
}

// Deep chains and many independent cycles exercise iterative traversal and
// guard against a full graph scan/allocation for each individual component.
// Rules: rules/analysis/deadlock_analysis.md §§58,82.
func TestLargeDependencyGraphs(t *testing.T) {
	const count = 30000
	for _, cycles := range []bool{false, true} {
		graph := Graph{Kind: WaitForGraph, Scope: "large"}
		for i := 0; i < count; i++ {
			graph.Nodes = append(graph.Nodes, NodeID(fmt.Sprintf("%06d", i)))
		}
		for i := 0; i < count-1; i++ {
			if !cycles || i%2 == 0 {
				graph.Edges = append(graph.Edges, Edge{EdgeID(fmt.Sprint(i)), graph.Nodes[i], graph.Nodes[i+1]})
			}
			if cycles && i%2 == 0 {
				graph.Edges = append(graph.Edges, Edge{EdgeID(fmt.Sprintf("back%d", i)), graph.Nodes[i+1], graph.Nodes[i]})
			}
		}
		got, err := Analyze(graph)
		want := count
		if cycles {
			want /= 2
		}
		if err != nil || len(got.Components) != want {
			t.Fatal(len(got.Components), err)
		}
		for _, component := range got.Components {
			if (len(component.Cycle) > 0) != cycles {
				t.Fatal(component)
			}
		}
	}
}

func checkResult(t *testing.T, graph Graph, result Result) {
	t.Helper()
	nodes := map[NodeID]int{}
	for i, n := range graph.Nodes {
		nodes[n] = i
	}
	reach := make([][]bool, len(nodes))
	for i := range reach {
		reach[i] = make([]bool, len(nodes))
		reach[i][i] = true
	}
	edges := map[EdgeID]Edge{}
	for _, e := range graph.Edges {
		reach[nodes[e.From]][nodes[e.To]] = true
		edges[e.ID] = e
	}
	for k := range reach {
		for i := range reach {
			for j := range reach {
				reach[i][j] = reach[i][j] || reach[i][k] && reach[k][j]
			}
		}
	}
	member := map[NodeID]int{}
	retained := map[EdgeID]int{}
	for i, c := range result.Components {
		for _, n := range c.Members {
			if _, duplicate := member[n]; duplicate {
				t.Fatal("duplicate node", n)
			}
			member[n] = i
		}
		for _, id := range c.InternalEdges {
			e, ok := edges[id]
			if !ok || !reach[nodes[e.From]][nodes[e.To]] || !reach[nodes[e.To]][nodes[e.From]] {
				t.Fatal("wrong internal evidence", id)
			}
			retained[id]++
		}
		for j, id := range c.Cycle {
			e, ok := edges[id]
			if !ok {
				t.Fatal("invented cycle edge", id)
			}
			next := edges[c.Cycle[(j+1)%len(c.Cycle)]]
			if e.To != next.From {
				t.Fatal("open cycle", c.Cycle)
			}
		}
	}
	if len(member) != len(nodes) {
		t.Fatal("lost nodes")
	}
	for a, i := range nodes {
		for b, j := range nodes {
			if (member[a] == member[b]) != (reach[i][j] && reach[j][i]) {
				t.Fatal("wrong SCC", a, b, result)
			}
		}
	}
	for _, a := range result.Condensation {
		if a.From == a.To {
			t.Fatal("condensation self-cycle", a)
		}
		for _, id := range a.Edges {
			retained[id]++
			e, ok := edges[id]
			if !ok || member[e.From] != a.From || member[e.To] != a.To {
				t.Fatal("wrong condensed evidence", a)
			}
		}
	}
	for id := range edges {
		if retained[id] != 1 {
			t.Fatal("lost or duplicated producer evidence", id, retained[id])
		}
	}
}
