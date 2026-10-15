package sema

import (
	"encoding/json"
	"errors"
	"sort"

	"sec/internal/lexer"
)

// StackRecursionDepthProof is an immutable producer-verified upper limit on
// simultaneously live invocations belonging to one entire canonical SCC,
// including its entry frame. It applies to every admitted entry/input and every
// internal transition, not just one decreasing edge or one cycle. This fact
// does not infer a ranking function or establish its initial-input bound.
// Rules: rules/analysis/stack_analysis.md — "Finite recursion and finite global bounds",
// "Recursive progress must cover relevant transitions", "Recursive SCC stack contribution".
type StackRecursionDepthProof struct {
	graph    *CallGraph
	snapshot string
	members  map[CallableID]bool
	level    StackMeasurementLevel
	plan     string
	depth    RecursionDepthBound
	source   lexer.Token
}

// NewVerifiedStackRecursionDepthProof consumes an authoritative whole-SCC depth
// proof and checks its structural coverage against the canonical graph. Covered
// call sites must include every internal same-stack invocation with all of its
// permitted targets. The producer owns the mathematical progress/input proof;
// listing edges alone is not evidence of termination. Facts are bound to this
// exact graph snapshot, level and plan and cannot be reused after graph changes.
// Rules: rules/analysis/stack_analysis.md — "Sources of finite recursion bounds",
// "Recursive progress must cover relevant transitions", "CompilationPlan dependence".
func NewVerifiedStackRecursionDepthProof(graph *CallGraph, entry CallableID, level StackMeasurementLevel, plan string, depth RecursionDepthBound, covered []CallSiteID, source lexer.Token) (*StackRecursionDepthProof, error) {
	count, finite := depth.Count()
	if graph == nil || !graph.IsSameStackRecursive(entry) || !finite || count.Sign() <= 0 ||
		(level != StackMeasurementSemantic && level != StackMeasurementMachine) || level == StackMeasurementMachine && plan == "" {
		return nil, errors.New("recursive stack proof requires a recursive SCC, positive finite depth and valid level/plan")
	}
	members := map[CallableID]bool{}
	for _, node := range graph.SameStackSCC(entry) {
		members[node.ID] = true
	}
	required := map[CallSiteID]bool{}
	for member := range members {
		for _, site := range graph.Outgoing(member) {
			if !sameStackExecution(site.Execution) {
				continue
			}
			for _, target := range site.Targets {
				if members[target] {
					required[site.ID] = true
				}
			}
		}
	}
	if len(required) == 0 || len(covered) != len(required) {
		return nil, errors.New("recursive depth proof does not cover every SCC transition")
	}
	for _, site := range covered {
		if !required[site] {
			return nil, errors.New("missing, duplicate or unrelated recursive depth proof transition")
		}
		delete(required, site)
	}
	return &StackRecursionDepthProof{graph, stackRecursionGraphStamp(graph), members, level, plan, depth, source}, nil
}

// stackRecursionGraphStamp invalidates proofs when graph structure, target
// coverage or represented effects change, including in-place mutation. Numeric
// frame costs remain separately supplied at the exact level/plan of composition.
// Rules: rules/analysis/stack_analysis.md — "Summary invalidation", "Call graph ownership".
func stackRecursionGraphStamp(graph *CallGraph) string {
	if graph == nil {
		return ""
	}
	data, _ := json.Marshal(struct{ Scope, Nodes, Bodies, Sites, Effects any }{graph.scope, graph.nodes, graph.bodyNodes, graph.sites, graph.effects})
	return graphIdentityDigest(data)
}

// matchingStackRecursionProof requires one unambiguous proof for this exact
// canonical component. Missing, conflicting, stale or wrong-domain proofs leave
// recursion Unknown instead of selecting an arbitrary smaller bound.
// Rules: rules/analysis/stack_analysis.md — "Unknown recursion depth",
// "CompilationPlan dependence", "Recursive progress must cover relevant transitions".
func matchingStackRecursionProof(graph *CallGraph, stamp string, members []CallableID, level StackMeasurementLevel, plan string, proofs []*StackRecursionDepthProof) *StackRecursionDepthProof {
	var selected *StackRecursionDepthProof
	for _, proof := range proofs {
		if proof == nil || proof.graph != graph || proof.snapshot != stamp || proof.level != level || proof.plan != plan || len(proof.members) != len(members) {
			continue
		}
		matches := true
		for _, member := range members {
			matches = matches && proof.members[member]
		}
		if !matches {
			continue
		}
		if selected != nil {
			return nil
		}
		selected = proof
	}
	return selected
}

// stackRecursionMembers retains a deterministic ordering independent of graph
// registration order for matrix coordinates and explanatory SCC evidence.
// Rules: rules/analysis/stack_analysis.md — "Determinism", "Recursive SCC stack contribution".
func stackRecursionMembers(proof *StackRecursionDepthProof) []CallableID {
	members := make([]CallableID, 0, len(proof.members))
	for member := range proof.members {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i] < members[j] })
	return members
}
