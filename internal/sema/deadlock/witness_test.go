package deadlock

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// Repeated witness searches inside one large SCC reuse visitation state safely
// and keep the bounded search iterative even when every return path is long.
// Rules: rules/analysis/deadlock_analysis.md §§59-60,82.
func TestRepresentativeLargeCycle(t *testing.T) {
	const size = 4096
	g := Graph{Kind: WaitForGraph, Scope: "ring"}
	for i := 0; i < size; i++ {
		g.Nodes = append(g.Nodes, NodeID(fmt.Sprintf("node%04d", i)))
	}
	byID := map[EdgeID]Edge{}
	for i := range g.Nodes {
		e := Edge{ID: EdgeID(fmt.Sprintf("edge%04d", i)), From: g.Nodes[i], To: g.Nodes[(i+1)%size]}
		g.Edges = append(g.Edges, e)
		byID[e.ID] = e
	}
	got, err := Analyze(g)
	if err != nil || len(got.Components) != 1 || len(got.Components[0].Cycle) != size {
		t.Fatal("large cyclic component", len(got.Components), err)
	}
	cycle := got.Components[0].Cycle
	for i, id := range cycle {
		if byID[id].To != byID[cycle[(i+1)%size]].From {
			t.Fatal("witness path does not close")
		}
	}
	if quality(cycle, got.Evidence).proof != 0 {
		t.Fatal("missing evidence gained proof")
	}
}

// Two cycles share a structural SCC but have independent semantic edge facts.
// The unknown bridge edges deliberately cannot support a strong witness.
func witnessFixture() Graph {
	g := Graph{Kind: WaitForGraph, Scope: "evidence", Nodes: []NodeID{"a", "b", "c", "d"}, Edges: []Edge{
		{"a1", "a", "b"}, {"a2", "b", "a"}, {"z1", "c", "d"}, {"z2", "d", "c"},
		{"bridge-out", "a", "c"}, {"bridge-back", "c", "a"},
	}, Evidence: map[EdgeID]EdgeEvidence{}}
	for _, id := range []EdgeID{"a1", "a2", "z1", "z2"} {
		g.Evidence[id] = EdgeEvidence{Strength: ProvenPossible, PreciseIdentity: true, File: "source.sec", Line: 1, Column: 1}
	}
	return g
}

func editEvidence(g *Graph, ids []EdgeID, edit func(*EdgeEvidence)) {
	for _, id := range ids {
		v := g.Evidence[id]
		edit(&v)
		g.Evidence[id] = v
	}
}

// Witness preferences consume the weakest proof in the complete cycle before
// length, identity precision, source clarity and uncertainty. No cycle ranking
// is a deadlock classification; an isolated strong edge cannot upgrade it.
// Rules: rules/analysis/deadlock_analysis.md §§8,49-51,59-60,82.
func TestRepresentativeCyclePreferences(t *testing.T) {
	first := []EdgeID{"a1", "a2"}
	for _, test := range []struct {
		name string
		edit func(*Graph)
		want []EdgeID
	}{
		{"equal evidence", func(*Graph) {}, first},
		{"stronger proof", func(g *Graph) { editEvidence(g, first, func(e *EdgeEvidence) { e.Strength = ConservativePossible }) }, []EdgeID{"z1", "z2"}},
		{"whole cycle bottleneck", func(g *Graph) {
			editEvidence(g, []EdgeID{"a2"}, func(e *EdgeEvidence) { e.Strength = Unknown })
			editEvidence(g, []EdgeID{"z1", "z2"}, func(e *EdgeEvidence) { e.Strength = ConservativePossible })
		}, []EdgeID{"z1", "z2"}},
		{"shorter explanation", func(g *Graph) {
			g.Nodes = append(g.Nodes, "e")
			g.Edges[1].To = "e"
			g.Edges = append(g.Edges, Edge{"a3", "e", "a"})
			g.Evidence["a3"] = g.Evidence["a1"]
		}, []EdgeID{"z1", "z2"}},
		{"proof before length", func(g *Graph) {
			g.Nodes = append(g.Nodes, "e")
			g.Edges[1].To = "e"
			g.Edges = append(g.Edges, Edge{"a3", "e", "a"})
			g.Evidence["a3"] = g.Evidence["a1"]
			editEvidence(g, []EdgeID{"z1", "z2"}, func(e *EdgeEvidence) { e.Strength = ConservativePossible })
		}, []EdgeID{"a1", "a2", "a3"}},
		{"precise identities", func(g *Graph) { editEvidence(g, first, func(e *EdgeEvidence) { e.PreciseIdentity = false }) }, []EdgeID{"z1", "z2"}},
		{"source clarity", func(g *Graph) { editEvidence(g, first, func(e *EdgeEvidence) { e.Column = 0 }) }, []EdgeID{"z1", "z2"}},
		{"less uncertainty", func(g *Graph) {
			editEvidence(g, first, func(e *EdgeEvidence) { e.Uncertainty = []string{"alias", "path"} })
		}, []EdgeID{"z1", "z2"}},
		{"source does not prove edge", func(g *Graph) { editEvidence(g, first, func(e *EdgeEvidence) { e.Strength = Unknown }) }, []EdgeID{"z1", "z2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := witnessFixture()
			test.edit(&g)
			random := rand.New(rand.NewSource(60))
			for round := 0; round < 30; round++ {
				random.Shuffle(len(g.Nodes), func(i, j int) { g.Nodes[i], g.Nodes[j] = g.Nodes[j], g.Nodes[i] })
				random.Shuffle(len(g.Edges), func(i, j int) { g.Edges[i], g.Edges[j] = g.Edges[j], g.Edges[i] })
				got, err := Analyze(g)
				if err != nil || len(got.Components) != 1 || !reflect.DeepEqual(got.Components[0].Cycle, test.want) {
					t.Fatalf("witness=%+v err=%v want=%v", got.Components, err, test.want)
				}
				checkResult(t, g, got)
			}
		})
	}
}

// Provenance and uncertainty are detached and order-insensitive. Missing or
// malformed metadata must not manufacture positive proof or partial results.
// Rules: rules/analysis/deadlock_analysis.md §§49-51,60,82.
func TestCycleEvidenceIsolationAndValidation(t *testing.T) {
	g := witnessFixture()
	editEvidence(&g, []EdgeID{"a1"}, func(e *EdgeEvidence) { e.Uncertainty = []string{"alias", "path", "alias"} })
	baseline, err := Analyze(g)
	if err != nil {
		t.Fatal(err)
	}
	editEvidence(&g, []EdgeID{"a1"}, func(e *EdgeEvidence) { e.Uncertainty = []string{"path", "alias"} })
	got, err := Analyze(g)
	if err != nil || !reflect.DeepEqual(baseline.Components, got.Components) {
		t.Fatal(got, err)
	}
	baseline.Evidence["a1"].Uncertainty[0] = "caller mutation"
	if g.Evidence["a1"].Uncertainty[0] == "caller mutation" {
		t.Fatal("evidence aliases input")
	}
	got.Evidence["z1"] = EdgeEvidence{Strength: Unknown}
	if g.Evidence["z1"].Strength != ProvenPossible {
		t.Fatal("map aliases input")
	}
	for _, bad := range []EdgeEvidence{{Strength: "fabricated"}, {Line: -1}, {Column: -1}} {
		g := witnessFixture()
		g.Evidence["a1"] = bad
		if got, err := Analyze(g); err == nil || !reflect.DeepEqual(got, Result{}) {
			t.Fatal(got, err)
		}
	}
	g = witnessFixture()
	g.Evidence["absent"] = EdgeEvidence{}
	if _, err := Analyze(g); err == nil {
		t.Fatal("dangling evidence accepted")
	}
	g = witnessFixture()
	g.Evidence = nil
	got, err = Analyze(g)
	if err != nil || quality(got.Components[0].Cycle, got.Evidence).proof != 0 {
		t.Fatal("missing evidence became proof", got, err)
	}
}
