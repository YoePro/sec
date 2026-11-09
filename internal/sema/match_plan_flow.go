package sema

// CanContinue reports whether execution may reach the match's continuation.
// It consumes resolved coverage and arm flow instead of reconstructing variant
// domains or inspecting source spellings. Missing coverage or unknown arm flow
// cannot prove termination, so incomplete plans conservatively may continue.
// Rules: rules/control-flow/flowcontrol_match.md §§5(2)-(4),23(1)-(3),24;
// rules/compiler/semantic_ir.md §71 (resolved match plans and explicit CFG).
func (plan ResolvedMatchPlan) CanContinue() bool {
	if !plan.Exhaustive || len(plan.Arms) == 0 {
		return true
	}
	for _, arm := range plan.Arms {
		switch arm.Flow {
		case MatchArmReturns, MatchArmTerminates, MatchArmLoopControl:
		default:
			return true
		}
	}
	return false
}
