package sema

import (
	"testing"

	"sec/internal/lexer"
)

// Reasons are provenance, not a hidden lattice encoding. An empty reason must
// not erase conditional certainty; prefix-like text cannot create uncertainty.
// Rules: rules/memory/ownership.md §§5–6,20–21.
func TestPlaceAvailabilityIndependentOfReason(t *testing.T) {
	token := lexer.Token{Line: 1, Column: 1}
	for _, tc := range []struct {
		name  string
		state unavailablePlaceState
		known bool
	}{
		{"ordinary definite", unavailableState("moved"), true},
		{"prefix is just provenance", unavailableState("conditional:moved"), true},
		{"conditional without reason", unavailablePlaceState{Availability: availabilityConditional}, false},
		{"conditional with reason", unavailablePlaceState{Availability: availabilityConditional, Reasons: "discarded"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAnalyzer()
			a.moved = map[string]lexer.Token{"value": token}
			a.availabilityStates = placeAvailabilityStates{"value": tc.state}
			known, available := a.currentPlaceAvailability(Place{Root: "value"})
			if known != tc.known || available {
				t.Fatalf("known=%t available=%t state=%+v", known, available, tc.state)
			}
		})
	}
}

// Continuing-path joins distinguish certainty from the union of ownership end
// reasons, including nested conditionals, dead paths and absent reason metadata.
// Rules: rules/memory/ownership.md §§5.6,6,20–21.
func TestPlaceAvailabilityJoins(t *testing.T) {
	token := lexer.Token{Line: 1, Column: 1}
	branch := func(state unavailablePlaceState) branchAnalysis {
		return branchAnalysis{continues: true, moved: map[string]lexer.Token{"value": token}, availabilityStates: placeAvailabilityStates{"value": state}}
	}
	moved := branch(unavailableState("moved"))
	discarded := branch(unavailableState("discarded"))
	conditional := branch(unavailablePlaceState{Availability: availabilityConditional, Reasons: "returned"})
	missingReason := branchAnalysis{continues: true, moved: map[string]lexer.Token{"value": token}}
	available := branchAnalysis{continues: true}
	dead := branch(unavailableState("released"))
	dead.continues = false
	for _, tc := range []struct {
		name      string
		branches  []branchAnalysis
		certainty placeAvailability
		reasons   string
	}{
		{"different causes every path", []branchAnalysis{moved, discarded}, availabilityUnavailable, "discarded|moved"},
		{"one available path", []branchAnalysis{moved, available}, availabilityConditional, "moved"},
		{"nested uncertainty", []branchAnalysis{conditional, discarded}, availabilityConditional, "discarded|returned"},
		{"dead path irrelevant", []branchAnalysis{moved, dead}, availabilityUnavailable, "moved"},
		{"reasonless conditional", []branchAnalysis{missingReason, available}, availabilityConditional, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			joined, states := mergeContinuingMoveState(nil, nil, tc.branches...)
			if len(joined) != 1 || states["value"].Availability != tc.certainty || states["value"].Reasons != tc.reasons {
				t.Fatalf("joined=%v states=%+v", joined, states)
			}
		})
	}
}

// Negative refinement changes certainty while preserving causes; positive
// refinement repairs only the related conditional masks, including children.
// Snapshot copies cannot mutate one another's availability or provenance.
// Rules: rules/memory/ownership.md §§20–21; ownership v2 negative refinement.
func TestPlaceAvailabilityRefinementAndSnapshots(t *testing.T) {
	a := NewAnalyzer()
	a.assigned = map[string]bool{}
	a.moved = map[string]lexer.Token{"value.field": {Line: 1}, "other.field": {Line: 2}}
	a.availabilityStates = placeAvailabilityStates{
		"value.field": {Availability: availabilityConditional, Reasons: "discarded|moved"},
		"other.field": {Availability: availabilityConditional, Reasons: "returned"},
	}
	before := copyAvailabilityStates(a.availabilityStates)
	a.applyAvailabilityRefinement(Place{Root: "value"}, false)
	if a.availabilityStates["value.field"].Availability != availabilityUnavailable || a.availabilityStates["value.field"].Reasons != "discarded|moved" || !isConditionallyUnavailable(a.availabilityStates["other.field"]) {
		t.Fatal(a.availabilityStates)
	}
	if !isConditionallyUnavailable(before["value.field"]) {
		t.Fatal("snapshot mutated")
	}
	a.availabilityStates = copyAvailabilityStates(before)
	a.applyAvailabilityRefinement(Place{Root: "value"}, true)
	if _, found := a.moved["value.field"]; found {
		t.Fatal("conditional child mask survived positive refinement")
	}
	if _, found := a.availabilityStates["value.field"]; found {
		t.Fatal("conditional child state survived positive refinement")
	}
	if !isConditionallyUnavailable(a.availabilityStates["other.field"]) {
		t.Fatal("unrelated state removed")
	}
}
