// Package deadlock analyzes producer-owned progress dependency graphs. It does
// not infer resource identity, graph coverage or cycle realizability from names.
package deadlock

import (
	"fmt"
	"sort"
)

// GraphKind keeps lock-order projections distinct from generalized wait-for
// graphs and from callable recursion graphs (deadlock_analysis.md §§7,45).
type GraphKind string

const (
	LockOrderGraph GraphKind = "lock-order"
	WaitForGraph   GraphKind = "wait-for"
)

// NodeID and EdgeID are opaque, snapshot-qualified producer identities, not
// binding names or an invitation to infer aliasing. EdgeID refers back to the
// producer's source, context, path, progress and proof-strength evidence.
// Rules: rules/analysis/deadlock_analysis.md §§47-50.
type NodeID string
type EdgeID string

type Edge struct {
	ID       EdgeID
	From, To NodeID
}

// ProofStrength describes an individual dependency, never a whole deadlock.
// Missing evidence remains Unknown, even in a graph-theoretic cycle.
// Rules: rules/analysis/deadlock_analysis.md §§8,49-51.
type ProofStrength string

const (
	Unknown              ProofStrength = ""
	ConservativePossible ProofStrength = "conservative-possible"
	ProvenPossible       ProofStrength = "proven-possible"
)

// EdgeEvidence is optional producer metadata used only for explanation choice.
// PreciseIdentity means the producer established the resource/context identity;
// source presence does not imply proof. Uncertainty retains localized causes.
// Rules: rules/analysis/deadlock_analysis.md §§49-50,60,82.
type EdgeEvidence struct {
	Strength        ProofStrength
	PreciseIdentity bool
	File            string
	Line, Column    int
	Uncertainty     []string
	// Cause names the last frame in a shared finite producer call path.
	// Empty means unavailable, never an invented direct-call proof.
	Cause CauseID
}

// Graph is one explicit producer snapshot. Isolated nodes remain observable;
// dangling endpoints and conflicting or duplicate identities are rejected.
// Missing edges never certify whole-program freedom from deadlock.
// Rules: rules/analysis/deadlock_analysis.md §§7-8,47-50,82.
type Graph struct {
	Kind     GraphKind
	Scope    string
	Nodes    []NodeID
	Edges    []Edge
	Evidence map[EdgeID]EdgeEvidence
	Causes   []CauseFrame
}

// Component is an SCC in canonical member order. InternalEdges retains all
// provenance identities, including parallel edges. Cycle is one deterministic
// closed edge walk when cyclic, empty for an acyclic singleton. It is only a
// structural candidate, with no deadlock classification or positive proof.
// Rules: rules/analysis/deadlock_analysis.md §§8,45,48-50,82.
type Component struct {
	Members       []NodeID
	InternalEdges []EdgeID
	Cycle         []EdgeID
}

// Arc is one condensation edge with all supporting original edge identities.
type Arc struct {
	From, To int
	Edges    []EdgeID
}

// Result is detached from its input and other calls. Component indices are
// stable within this result, ordered by their smallest canonical member.
// Condensation is a DAG; its edges never erase supporting edge identities.
// Rules: rules/analysis/deadlock_analysis.md §§45,48-50,82.
type Result struct {
	Kind         GraphKind
	Scope        string
	Edges        []Edge
	Components   []Component
	Condensation []Arc
	Evidence     map[EdgeID]EdgeEvidence
	// Causes retains only call-path prefixes needed by representative edges.
	Causes []CauseFrame
}

// Analyze computes SCCs, a condensation DAG and one candidate witness per
// cyclic component. Both graph kinds use the same deterministic algorithm,
// without transferring call-graph SCCs or upgrading cycles to deadlock proofs.
// Graph traversals are iterative to handle deep dependency chains safely.
// Rules: rules/analysis/deadlock_analysis.md §§7-8,45,48-50,58,82.
func Analyze(graph Graph) (Result, error) {
	if graph.Kind != LockOrderGraph && graph.Kind != WaitForGraph {
		return Result{}, fmt.Errorf("unsupported dependency graph kind %q", graph.Kind)
	}
	if graph.Scope == "" {
		return Result{}, fmt.Errorf("dependency graph requires a producer scope")
	}
	nodes := append([]NodeID(nil), graph.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	index := make(map[NodeID]int, len(nodes))
	for i, node := range nodes {
		if node == "" || (i > 0 && nodes[i-1] == node) {
			return Result{}, fmt.Errorf("empty or duplicate dependency node %q", node)
		}
		index[node] = i
	}
	edges := append([]Edge(nil), graph.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.ID < b.ID
	})
	out, reverse := make([][]int, len(nodes)), make([][]int, len(nodes))
	seen := make(map[EdgeID]bool, len(edges))
	for i, edge := range edges {
		from, fromOK := index[edge.From]
		to, toOK := index[edge.To]
		if edge.ID == "" || seen[edge.ID] || !fromOK || !toOK {
			return Result{}, fmt.Errorf("invalid, duplicate or dangling dependency edge %q", edge.ID)
		}
		seen[edge.ID] = true
		out[from], reverse[to] = append(out[from], i), append(reverse[to], from)
	}
	evidence := make(map[EdgeID]EdgeEvidence, len(graph.Evidence))
	var evidenceIDs []EdgeID
	for id := range graph.Evidence {
		evidenceIDs = append(evidenceIDs, id)
	}
	sort.Slice(evidenceIDs, func(i, j int) bool { return evidenceIDs[i] < evidenceIDs[j] })
	for _, id := range evidenceIDs {
		fact := graph.Evidence[id]
		if !seen[id] || (fact.Strength != Unknown && fact.Strength != ConservativePossible && fact.Strength != ProvenPossible) || fact.Line < 0 || fact.Column < 0 {
			return Result{}, fmt.Errorf("invalid dependency evidence for edge %q", id)
		}
		fact.Uncertainty = append([]string(nil), fact.Uncertainty...)
		evidence[id] = fact
	}
	causes, err := validateCauses(graph.Causes, evidence)
	if err != nil {
		return Result{}, err
	}
	groups := components(nodes, edges, out, reverse, index)
	result := Result{Kind: graph.Kind, Scope: graph.Scope, Edges: edges, Evidence: evidence}
	membership := make([]int, len(nodes))
	for i, group := range groups {
		component := Component{}
		for _, node := range group {
			membership[node] = i
			component.Members = append(component.Members, nodes[node])
		}
		result.Components = append(result.Components, component)
	}
	internal := make([][]int, len(groups))
	type pair struct{ from, to int }
	arcs := map[pair][]EdgeID{}
	for i, edge := range edges {
		from, to := membership[index[edge.From]], membership[index[edge.To]]
		if from == to {
			internal[from] = append(internal[from], i)
			result.Components[from].InternalEdges = append(result.Components[from].InternalEdges, edge.ID)
		} else {
			key := pair{from, to}
			arcs[key] = append(arcs[key], edge.ID)
		}
	}
	for key, causes := range arcs {
		result.Condensation = append(result.Condensation, Arc{From: key.from, To: key.to, Edges: causes})
	}
	sort.Slice(result.Condensation, func(i, j int) bool {
		a, b := result.Condensation[i], result.Condensation[j]
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	for i, group := range groups {
		if len(group) > 1 || (len(internal[i]) > 0 && edges[internal[i][0]].From == edges[internal[i][0]].To) {
			result.Components[i].Cycle = representativeCycle(result.Components[i].Members, internal[i], edges, evidence)
		}
	}
	result.Causes = retainCycleCauses(result.Components, evidence, causes)
	return result, nil
}
