package sema

import "sort"

// SameStackComponent is a detached canonical SCC and its recursion fact. An
// isolated singleton is not recursive; a singleton with a self-edge is.
// Rules: rules/analysis/call_graph.md — "Recursion", Appendix A.20 "SCC views";
// rules/analysis/stack_analysis.md — "Call graph ownership", "Recursion and SCCs".
type SameStackComponent struct {
	Members   []CallableID
	Recursive bool
}

// SameStackComponents exposes the canonical same-stack SCC partition. Targets
// come only from the existing graph view (including represented callbacks and
// reentry); spawned workers retain their distinct execution relationships.
// Components and members are ordered by semantic ID, independent of graph
// registration order. Every result is detached from the graph and other calls.
// Unknown/open targets are never invented to complete a component.
// Rules: rules/analysis/call_graph.md — "One canonical graph, multiple analysis views", "Same-stack execution", "Recursion", Appendix A.20;
// rules/analysis/stack_analysis.md — "Call graph ownership", "Recursion and SCCs", "Determinism".
func (g *CallGraph) SameStackComponents() []SameStackComponent {
	if g == nil {
		return nil
	}
	// Classify singleton self-edges once, without rescanning every call site
	// for every isolated component in large workspace graphs.
	selfEdges := map[CallableID]bool{}
	for _, site := range g.sites {
		if !sameStackExecution(site.Execution) {
			continue
		}
		for _, target := range site.Targets {
			if target == site.Caller {
				selfEdges[target] = true
			}
		}
	}
	var result []SameStackComponent
	for _, members := range g.sameStackComponents() {
		component := SameStackComponent{Recursive: len(members) > 1}
		for id := range members {
			component.Members = append(component.Members, id)
		}
		sort.Slice(component.Members, func(i, j int) bool { return component.Members[i] < component.Members[j] })
		if len(component.Members) == 1 {
			component.Recursive = selfEdges[component.Members[0]]
		}
		result = append(result, component)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Members[0] < result[j].Members[0] })
	return result
}

// SameStackSCC returns one canonical component in the existing node display
// order while sharing the batch view's membership and recursion ownership.
// Rules: rules/analysis/call_graph.md — "Recursion", Appendix A.20 "SCC views".
func (g *CallGraph) SameStackSCC(id CallableID) []CallableNode {
	if g == nil {
		return nil
	}
	for _, component := range g.SameStackComponents() {
		for _, member := range component.Members {
			if member != id {
				continue
			}
			included := map[CallableID]bool{}
			for _, member := range component.Members {
				included[member] = true
			}
			return g.nodesInOrder(included)
		}
	}
	return nil
}

// IsSameStackRecursive consumes the canonical component recursion fact; an
// absent node or isolated singleton cannot gain a recursive stack obligation.
// Rules: rules/analysis/call_graph.md — "Recursion", Appendix A.20;
// rules/analysis/stack_analysis.md — "Recursion and SCCs".
func (g *CallGraph) IsSameStackRecursive(id CallableID) bool {
	for _, component := range g.SameStackComponents() {
		for _, member := range component.Members {
			if member == id {
				return component.Recursive
			}
		}
	}
	return false
}
