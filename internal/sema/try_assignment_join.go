package sema

import (
	"strings"

	"sec/internal/ast"
)

// tryAssignmentHandlerBranches analyzes the handlers of one try assignment as
// separate branches of its failure edge. Every handler starts from the
// failure-edge state, so a move, assignment or borrow in one handler is never
// visible to another handler or to the success continuation.
type tryAssignmentHandlerBranches struct {
	entry    branchAnalysis
	branches []branchAnalysis
}

// analyze runs one handler body from the failure-edge state and records its
// end state. Only a handler whose body completes normally continues after the
// statement; returning, propagating and terminating handlers leave it.
func (c *tryAssignmentHandlerBranches) analyze(a *Analyzer, body func() ResolvedTryHandlerFlow) ResolvedTryHandlerFlow {
	a.applyBranchAnalysisState(c.entry)
	var origins map[string]Type
	a.tryHandlerOrigins = &origins
	flow := body()
	a.tryHandlerOrigins = nil
	branch := a.currentBranchAnalysisState()
	branch.referenceOrigins = origins
	branch.continues = flow == TryHandlerProducesValue
	c.branches = append(c.branches, branch)
	a.applyBranchAnalysisState(c.entry)
	return flow
}

// tryAssignmentFailureEntry derives the failure-edge state from the success
// state: the assigned value has been evaluated on both edges, but the target
// keeps its pre-statement initialization, move, borrow and reference-origin
// state because a rejected assignment never stores into it.
//
// Rules:
//   - rules/errors/errorhandling.md — §23 "Fallible assignment"
//   - rules/memory/ownership.md — §§20–21 control-flow merge
func (a *Analyzer) tryAssignmentFailureEntry(target ast.Expression, before, success branchAnalysis) branchAnalysis {
	failure := success
	failure.assigned = copyAssigned(success.assigned)
	failure.moved = copyMoved(success.moved)
	failure.moveReasons = copyMoveReasons(success.moveReasons)
	failure.borrows = copyBorrows(success.borrows)
	failure.localRefContainers = copyLocalRefContainers(success.localRefContainers)
	failure.referenceOrigins = copyReferenceOriginTypes(success.referenceOrigins)
	place, ok := a.resolvePlace(target)
	if !ok {
		return failure
	}
	key := place.String()
	if key == place.Root {
		if assigned, tracked := before.assigned[key]; tracked {
			failure.assigned[key] = assigned
		} else {
			delete(failure.assigned, key)
		}
	}
	covered := func(candidate string) bool {
		return candidate == key || strings.HasPrefix(candidate, key+".") || strings.HasPrefix(candidate, key+"[")
	}
	for moved := range failure.moved {
		if covered(moved) {
			delete(failure.moved, moved)
			delete(failure.moveReasons, moved)
		}
	}
	for moved, token := range before.moved {
		if covered(moved) {
			failure.moved[moved] = token
			if reason, ok := before.moveReasons[moved]; ok {
				failure.moveReasons[moved] = reason
			}
		}
	}
	if symbol, ok := a.symbols[place.Root]; ok && key == place.Root && typeCarriesReferenceOrigin(symbol.Type) {
		// A reference holder keeps its previous referent: the borrows it held
		// and its origin are those from before the statement.
		failure.borrows = copyBorrows(before.borrows)
		if origin, ok := before.localRefContainers[key]; ok {
			failure.localRefContainers[key] = origin
		} else {
			delete(failure.localRefContainers, key)
		}
		if origin, ok := before.referenceOrigins[key]; ok {
			failure.referenceOrigins[key] = origin
		} else {
			delete(failure.referenceOrigins, key)
		}
	}
	return failure
}

// joinTryAssignmentEdges merges the success continuation with every handler
// that completes normally. Residual propagation and returning or terminating
// handlers leave the statement and do not reach the join.
func (a *Analyzer) joinTryAssignmentEdges(before branchAnalysis, branches ...branchAnalysis) {
	a.assigned = mergeContinuingAssigned(before.assigned, branches...)
	a.moved, a.moveReasons = mergeContinuingMoveState(before.moved, before.moveReasons, branches...)
	a.closedResources = mergeContinuingClosedResources(before.closedResources, branches...)
	a.borrows = mergeContinuingBorrows(before.borrows, branches...)
	a.localRefContainers = mergeContinuingLocalRefContainers(before.localRefContainers, branches...)
	a.arenaGenerations = mergeContinuingArenaGenerations(before.arenaGenerations, branches...)
	a.mergeContinuingReferenceOrigins(branches...)
}

func copyReferenceOriginTypes(origins map[string]Type) map[string]Type {
	if origins == nil {
		return map[string]Type{}
	}
	out := make(map[string]Type, len(origins))
	for name, origin := range origins {
		out[name] = origin
	}
	return out
}
