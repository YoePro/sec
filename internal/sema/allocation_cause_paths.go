package sema

import (
	"fmt"
	"sec/internal/lexer"
)

// AllocationCauseStep links a policy violation to a canonical root, call site,
// callee declaration or introducing operation. No relationship is inferred from
// names; absent source metadata stays explicitly incomplete.
// Rules: rules/memory/allocation.md — §§24(6),28(4),29(4).
type AllocationCauseStep struct {
	Kind           string
	Source         lexer.Token
	Message        string
	Caller, Callee CallableID
	Site           CallSiteID
}

// AllocationCausePath keeps definite allocation distinct from unknown foreign
// or indirect behavior, with a deterministic synchronous witness.
// Rules: rules/memory/allocation.md — §§24(6),28(4),29(4).
type AllocationCausePath struct {
	Unknown    bool
	Incomplete bool
	Path       []CallableID
	Steps      []AllocationCauseStep
}

// AllocationCause reconstructs navigable source evidence for the same shortest
// path used by @noAlloc validation. Same-stack defer participates; spawned work
// cannot be substituted for a synchronous cause. Returned slices are detached.
// Rules: rules/memory/allocation.md — §§24,28(4),29(1),(4);
// rules/analysis/call_graph.md — "Same-stack execution", "Cause paths".
func (g *CallGraph) AllocationCause(id CallableID) AllocationCausePath {
	summary := g.ArenaSummary(id)
	result := AllocationCausePath{}
	path := summary.AllocationPath
	if !summary.MayAllocate && !summary.AllocationUnknown {
		return result
	}
	if !summary.MayAllocate {
		result.Unknown = true
		path = summary.UnknownAllocationPath
	}
	result.Path = append([]CallableID(nil), path...)
	if len(path) == 0 {
		return result
	}
	add := func(step AllocationCauseStep) {
		if step.Source.File == "" || step.Source.Line <= 0 || step.Source.Column <= 0 {
			result.Incomplete = true
		}
		result.Steps = append(result.Steps, step)
	}
	root, _ := g.Node(path[0])
	add(AllocationCauseStep{Kind: "root", Source: root.Declaration, Caller: root.ID, Message: "allocation policy root: " + root.Name})
	for i := 1; i < len(path); i++ {
		from, _ := g.Node(path[i-1])
		to, _ := g.Node(path[i])
		found := false
		for _, site := range g.Outgoing(from.ID) {
			if !sameStackExecution(site.Execution) {
				continue
			}
			for _, target := range site.Targets {
				if target != to.ID {
					continue
				}
				add(AllocationCauseStep{Kind: "call", Source: site.Source, Caller: from.ID, Callee: to.ID, Site: site.ID, Message: fmt.Sprintf("%s reaches %s through %s execution", from.Name, to.Name, site.Execution)})
				found = true
				break
			}
			if found {
				break
			}
		}
		if !found {
			result.Incomplete = true
		}
	}
	last, _ := g.Node(path[len(path)-1])
	add(AllocationCauseStep{Kind: "callee", Source: last.Declaration, Caller: last.ID, Message: "allocation effect originates in " + last.Name})
	site, found := introducingAllocationSite(g.ArenaSummary(last.ID).DirectEffects, result.Unknown)
	if found {
		kind := "allocation operation"
		if result.Unknown {
			kind = "unresolved allocation behavior"
		}
		add(AllocationCauseStep{Kind: "operation", Source: site.Source, Caller: last.ID, Message: kind + ": " + string(site.Kind)})
	} else {
		result.Incomplete = true
	}
	return result
}
