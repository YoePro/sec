package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"strings"
)

// placeAvailability separates continuing-path certainty from why ownership
// ended. This sparse map accompanies moved Places; available/uninitialized
// roots and partial aggregates retain the existing assigned/projection model.
// Reasons are diagnostic provenance and never encode an availability state.
// Rules: rules/memory/ownership.md §§5–6,20–21; rules/memory/destruction.md §11.1.
type placeAvailability uint8

const (
	availabilityUnavailable placeAvailability = iota
	availabilityConditional
)

type unavailablePlaceState struct {
	Availability placeAvailability
	Reasons      string
}

type placeAvailabilityStates map[string]unavailablePlaceState

// unavailableState records a definite ownership end independently of its cause.
// Rules: rules/memory/ownership.md §§5.5,6.
func unavailableState(reasons string) unavailablePlaceState {
	return unavailablePlaceState{Availability: availabilityUnavailable, Reasons: reasons}
}

// isConditionallyUnavailable tests the availability dimension, never prose.
// Rules: rules/memory/ownership.md §5.6.
func isConditionallyUnavailable(state unavailablePlaceState) bool {
	return state.Availability == availabilityConditional
}

// copyAvailabilityStates detaches a flow snapshot; all record fields are values.
// Rules: rules/memory/ownership.md §§20–21.
func copyAvailabilityStates(in placeAvailabilityStates) placeAvailabilityStates {
	out := make(placeAvailabilityStates, len(in))
	for place, state := range in {
		out[place] = state
	}
	return out
}

// mergeContinuingMoveState retains every reason as provenance and separately
// joins certainty. Non-continuing paths cannot make a continuing Place
// conditional; conditional input stays conditional through nested joins.
// Rules: rules/memory/ownership.md §§5–6,20–21.
func mergeContinuingMoveState(beforeMoved map[string]lexer.Token, beforeStates placeAvailabilityStates, branches ...branchAnalysis) (map[string]lexer.Token, placeAvailabilityStates) {
	moved := map[string]lexer.Token{}
	states := placeAvailabilityStates{}
	present := map[string]int{}
	reasons := map[string][]string{}
	conditional := map[string]bool{}
	paths := 0
	for _, branch := range branches {
		if !branch.continues {
			continue
		}
		paths++
		for place, token := range branch.moved {
			present[place]++
			state := branch.availabilityStates[place]
			reasons[place] = append(reasons[place], state.Reasons)
			conditional[place] = conditional[place] || isConditionallyUnavailable(state)
			if _, exists := moved[place]; !exists {
				moved[place] = token
			}
		}
	}
	if paths == 0 {
		return copyMoved(beforeMoved), copyAvailabilityStates(beforeStates)
	}
	for place := range moved {
		state := unavailableState(joinAvailabilityReasons(reasons[place]))
		if present[place] < paths || conditional[place] {
			state.Availability = availabilityConditional
		}
		states[place] = state
	}
	return moved, states
}

// currentPlaceAvailability projects stored certainty for an ownership query.
// Rules: rules/memory/ownership.md §§5,20–21.
func (a *Analyzer) currentPlaceAvailability(place Place) (known bool, available bool) {
	if assigned, tracked := a.assigned[place.Root]; tracked && !assigned {
		return true, false
	}
	_, movedKey, _, unavailable := a.unavailablePlace(place)
	if !unavailable {
		return true, true
	}
	if isConditionallyUnavailable(a.availabilityStates[movedKey]) {
		return false, false
	}
	return true, false
}

// analyzeAvailabilityBranchWithCallGraphReachability applies only the branch
// refinement proven by the ownership query and restores the incoming analysis
// state after capturing the branch result.
//
// Rules:
//   - rules/memory/ownership.md — §§20–21 control-flow merge and refinement
func (a *Analyzer) analyzeAvailabilityBranchWithCallGraphReachability(block *ast.BlockStatement, fact ResolvedAvailabilityTest, refineAvailable bool, reachable bool) branchAnalysis {
	previousMoved, previousReasons := a.moved, a.availabilityStates
	previousAssigned := a.assigned
	a.moved, a.availabilityStates = copyMoved(previousMoved), copyAvailabilityStates(previousReasons)
	a.assigned = copyAssigned(previousAssigned)
	a.applyAvailabilityRefinement(fact.Place, refineAvailable)
	branch := a.analyzeBranchBlockWithCallGraphReachability(block, reachable)
	a.moved, a.availabilityStates, a.assigned = previousMoved, previousReasons, previousAssigned
	return branch
}

// refinedAvailabilityFallthrough refines only the continuing branch snapshot
// and restores the caller's flow state after capturing the result.
// Rules: rules/memory/ownership.md §§20–21.
func (a *Analyzer) refinedAvailabilityFallthrough(branch branchAnalysis, fact ResolvedAvailabilityTest, refineAvailable bool) branchAnalysis {
	previousMoved, previousReasons := a.moved, a.availabilityStates
	previousAssigned := a.assigned
	a.moved, a.availabilityStates = copyMoved(branch.moved), copyAvailabilityStates(branch.availabilityStates)
	a.assigned = copyAssigned(branch.assigned)
	a.applyAvailabilityRefinement(fact.Place, refineAvailable)
	branch.moved, branch.availabilityStates, branch.assigned = copyMoved(a.moved), copyAvailabilityStates(a.availabilityStates), copyAssigned(a.assigned)
	a.moved, a.availabilityStates, a.assigned = previousMoved, previousReasons, previousAssigned
	return branch
}

// applyAvailabilityRefinement repairs related conditional masks on a positive
// test and makes only existing related masks definite on a negative test. It
// never fabricates a wholly moved owner from a partially unavailable aggregate.
// Rules: rules/memory/ownership.md §§5.4–5.6,20–21.
func (a *Analyzer) applyAvailabilityRefinement(place Place, available bool) {
	if available {
		key := place.String()
		for movedKey, reason := range a.availabilityStates {
			if !isConditionallyUnavailable(reason) {
				continue
			}
			if movedKey == key || strings.HasPrefix(key, movedKey+".") || strings.HasPrefix(key, movedKey+"[") || strings.HasPrefix(movedKey, key+".") || strings.HasPrefix(movedKey, key+"[") {
				delete(a.moved, movedKey)
				delete(a.availabilityStates, movedKey)
			}
		}
		if place.String() == place.Root {
			a.assigned[place.Root] = true
		}
		return
	}
	for movedKey, reason := range a.availabilityStates {
		if isConditionallyUnavailable(reason) && (movedKey == place.String() || strings.HasPrefix(movedKey, place.String()+".") || strings.HasPrefix(movedKey, place.String()+"[")) {
			reason.Availability = availabilityUnavailable
			a.availabilityStates[movedKey] = reason
		}
	}
}
