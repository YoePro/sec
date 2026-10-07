package sema

// arenaAllocationDistance keeps independent finite-lattice allocation and
// unknown facts. Negative distances denote absence; nonnegative distances
// retain shortest witnesses without traversing a recursive body repeatedly.
// Rules: rules/memory/allocation.md — §§6(3)-(4),24(2),(6);
// rules/analysis/effect_analysis.md — "Fixed-point analysis".
type arenaAllocationDistance struct{ allocating, unknown int }

type arenaAllocationFixedPoint struct {
	distance map[CallableID]arenaAllocationDistance
	targets  map[CallableID][]CallableID
}

// ArenaSummary consumes the graph-wide converged allocation summary, retaining
// source-ordered direct events and detached deterministic shortest witnesses.
// A cycle alone supplies neither a positive effect nor an unknown effect;
// incomplete operation/body coverage remains owned by AllocationFacts.
// Rules: rules/memory/allocation.md — §§6(3)-(4),24(2),(4),(6);
// rules/analysis/call_graph.md — "Same-stack effect transfer", "Spawn effect transfer", A.21.
func (g *CallGraph) ArenaSummary(id CallableID) ArenaCallableSummary {
	if g == nil {
		return ArenaCallableSummary{}
	}
	state := g.convergedAllocationFixedPoint()
	distance, exists := state.distance[id]
	summary := ArenaCallableSummary{DirectEffects: append([]ArenaEffectSite(nil), g.arenaEffects[id]...)}
	if !exists {
		return summary
	}
	summary.MayAllocate = distance.allocating >= 0
	summary.AllocationUnknown = distance.unknown >= 0
	summary.AllocationPath = state.path(id, false)
	summary.UnknownAllocationPath = state.path(id, true)
	return summary
}

// invalidateAllocationFixedPoint replaces derived summaries whenever a direct
// allocation site or execution edge changes during graph construction. Cloning,
// plan binding and checkpoint restoration construct fresh uncached graphs.
// Rules: rules/memory/allocation.md — §§24(4),29(5).
func (g *CallGraph) invalidateAllocationFixedPoint() {
	g.allocationMu.Lock()
	g.allocationFixedPoint = nil
	g.allocationMu.Unlock()
}

// convergedAllocationFixedPoint solves every recursive component with an
// unbounded-by-analysis-depth monotone reverse worklist. Each distance changes
// from absent to finite and can only decrease; cycles with no seed stay absent.
// Independent seeds prevent an unknown path from erasing definite allocation.
// Only same-stack relations transfer effects; spawn bodies retain their own
// summaries. Cache publication is safe for concurrent immutable-view consumers.
// Rules: rules/analysis/effect_analysis.md — "Fixed-point analysis";
// rules/analysis/call_graph.md — A.21, "Same-stack effect transfer", "Spawn effect transfer";
// rules/memory/allocation.md — §§6(3)-(4),24(4),(6).
func (g *CallGraph) convergedAllocationFixedPoint() *arenaAllocationFixedPoint {
	g.allocationMu.Lock()
	defer g.allocationMu.Unlock()
	if g.allocationFixedPoint != nil {
		return g.allocationFixedPoint
	}
	state := &arenaAllocationFixedPoint{distance: map[CallableID]arenaAllocationDistance{}, targets: map[CallableID][]CallableID{}}
	predecessors := map[CallableID][]CallableID{}
	seenEdges := map[CallableID]map[CallableID]bool{}
	for _, id := range g.nodeOrder {
		state.distance[id] = arenaAllocationDistance{-1, -1}
	}
	for _, site := range g.sites {
		if !sameStackExecution(site.Execution) {
			continue
		}
		for _, target := range site.Targets {
			if target == "" {
				continue
			}
			if seenEdges[site.Caller] == nil {
				seenEdges[site.Caller] = map[CallableID]bool{}
			}
			if seenEdges[site.Caller][target] {
				continue
			}
			seenEdges[site.Caller][target] = true
			state.targets[site.Caller] = append(state.targets[site.Caller], target)
			predecessors[target] = append(predecessors[target], site.Caller)
		}
	}
	queue := []CallableID{}
	queued := map[CallableID]bool{}
	for _, id := range g.nodeOrder {
		distance := state.distance[id]
		for _, effect := range g.arenaEffects[id] {
			if effect.MayAllocate {
				distance.allocating = 0
			}
			if effect.UnknownAllocation {
				distance.unknown = 0
			}
		}
		state.distance[id] = distance
		if distance.allocating >= 0 || distance.unknown >= 0 {
			queue = append(queue, id)
			queued[id] = true
		}
	}
	for head := 0; head < len(queue); head++ {
		target := queue[head]
		queued[target] = false
		callee := state.distance[target]
		for _, caller := range predecessors[target] {
			distance, exists := state.distance[caller]
			if !exists {
				continue
			}
			before := distance
			if callee.allocating >= 0 && (distance.allocating < 0 || callee.allocating+1 < distance.allocating) {
				distance.allocating = callee.allocating + 1
			}
			if callee.unknown >= 0 && (distance.unknown < 0 || callee.unknown+1 < distance.unknown) {
				distance.unknown = callee.unknown + 1
			}
			if distance == before {
				continue
			}
			state.distance[caller] = distance
			if !queued[caller] {
				queue = append(queue, caller)
				queued[caller] = true
			}
		}
	}
	g.allocationFixedPoint = state
	return state
}

// path reconstructs a shortest witness by following strictly decreasing
// distances. Canonical edge order resolves ties exactly as source-ordered BFS,
// independent of worklist processing order; cycles cannot enter the witness.
// Rules: rules/memory/allocation.md — §§24(4),28(4),29(4);
// rules/analysis/call_graph.md — "Diagnostics".
func (s *arenaAllocationFixedPoint) path(id CallableID, unknown bool) []CallableID {
	distance := func(id CallableID) int {
		value, exists := s.distance[id]
		if !exists {
			return -1
		}
		if unknown {
			return value.unknown
		}
		return value.allocating
	}
	remaining := distance(id)
	if remaining < 0 {
		return nil
	}
	path := []CallableID{id}
	for remaining > 0 {
		for _, target := range s.targets[id] {
			if distance(target) == remaining-1 {
				id = target
				break
			}
		}
		path = append(path, id)
		remaining--
	}
	return path
}
