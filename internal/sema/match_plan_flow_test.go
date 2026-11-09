package sema

import "testing"

// TestMatchContinuationRequiresCompleteFlow protects the common Sema/IR
// boundary against mistaking absent/future flow facts for a termination proof.
// Rules: rules/control-flow/flowcontrol_match.md §§5,23(2)-(3).
func TestMatchContinuationRequiresCompleteFlow(t *testing.T) {
	tests := []struct {
		name       string
		exhaustive bool
		flows      []ResolvedMatchArmFlow
		continues  bool
	}{
		{"missing coverage", false, []ResolvedMatchArmFlow{MatchArmReturns}, true},
		{"missing arms", true, nil, true},
		{"missing flow", true, []ResolvedMatchArmFlow{MatchArmReturns, ""}, true},
		{"future flow", true, []ResolvedMatchArmFlow{MatchArmReturns, "future"}, true},
		{"all exits", true, []ResolvedMatchArmFlow{MatchArmReturns, MatchArmTerminates, MatchArmLoopControl}, false},
		{"continuing arm", true, []ResolvedMatchArmFlow{MatchArmReturns, MatchArmContinues}, true},
		{"value merge", true, []ResolvedMatchArmFlow{MatchArmReturns, MatchArmProducesValue}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ResolvedMatchPlan{Exhaustive: tt.exhaustive}
			for _, flow := range tt.flows {
				plan.Arms = append(plan.Arms, ResolvedMatchArm{Flow: flow})
			}
			if got := plan.CanContinue(); got != tt.continues {
				t.Fatalf("CanContinue() = %v, want %v", got, tt.continues)
			}
		})
	}
}
