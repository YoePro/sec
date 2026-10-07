package sema

import (
	"sec/internal/ast"
)

// analyzeIfStatement applies canonical condition, Option binding, availability
// and variant facts independently to reachable branches, then merges only the
// continuing ownership/state paths. Result checks retain their source proof.
// Rules: rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements";
// rules/declarations/unions.md — §§8.1–8.4;
// rules/errors/errorhandling.md — §§6.1–6.2.
func (a *Analyzer) analyzeIfStatement(stmt *ast.IfStatement) {
	var optionBinding matchPatternInfo
	optionBindingValid := false
	var availabilityTest *ResolvedAvailabilityTest
	var stateTest *ResolvedStateTest
	constantCondition := false
	constantConditionKnown := false
	if stmt.OptionBinding != nil {
		optionBinding, optionBindingValid = a.resolveOptionIfBinding(stmt)
	} else if stmt.Condition != nil {
		conditionType, _ := a.inferExpression(stmt.Condition)
		if conditionType.Kind != InvalidType && conditionType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(stmt.Condition), "%s", nonBoolConditionMessage(stmt.Condition, "if", conditionType))
		}
		if conditionType.Kind == BoolType {
			constantCondition, constantConditionKnown = a.constantBooleanValue(stmt.Condition)
		}
		if availabilityExpr, ok := stmt.Condition.(*ast.AvailabilityExpression); ok {
			if fact, resolved := a.resolvedAvailabilityTests[availabilityExpr]; resolved {
				availabilityTest = &fact
			}
		}
		if stateExpr, ok := stmt.Condition.(*ast.StateTestExpression); ok {
			if fact, resolved := a.resolvedStateTests[stateExpr]; resolved {
				stateTest = &fact
			}
		}
	}

	before := copyAssigned(a.assigned)
	beforeMoved := copyMoved(a.moved)
	beforeMoveReasons := copyMoveReasons(a.moveReasons)
	beforeClosedResources := copyMoved(a.closedResources)
	beforeBorrows := copyBorrows(a.borrows)
	beforeLocalRefContainers := copyLocalRefContainers(a.localRefContainers)
	beforeArenaGenerations := copyArenaGenerations(a.arenaGenerations)
	thenReachable := true
	elseReachable := true
	if constantConditionKnown && constantCondition {
		elseReachable = false
		a.diagnoseConstantConditionUnreachableBlock(stmt.Alternative, "branch")
	} else if constantConditionKnown {
		thenReachable = false
		a.diagnoseConstantConditionUnreachableBlock(stmt.Consequence, "branch")
	} else if availabilityTest != nil && availabilityTest.StaticallyKnown {
		thenReachable = availabilityTest.Value
		elseReachable = !availabilityTest.Value
	} else if stateTest != nil && stateTest.StaticallyKnown {
		thenReachable = stateTest.Value
		elseReachable = !stateTest.Value
		switch {
		case stateTest.Empty && !stateTest.Value:
			a.diagnoseImpossibleStateTestBlock(stmt.Consequence, stateTest.Binding)
		case !stateTest.Empty && !stateTest.Value:
			a.diagnoseKnownVariantBlock(stmt.Consequence, *stateTest)
		case !stateTest.Empty:
			a.diagnoseKnownVariantBlock(stmt.Alternative, *stateTest)
		}
	} else if stmt.OptionBinding == nil {
		if value, known := a.relationalConditionValue(stmt.Condition); known {
			thenReachable = value
			elseReachable = !value
			if value {
				a.diagnoseRelationallyUnreachableBlock(stmt.Alternative, true, "branch")
			} else {
				a.diagnoseRelationallyUnreachableBlock(stmt.Consequence, false, "branch")
			}
		}
	}
	refinementCount := len(a.activeConditionFacts)
	if stmt.OptionBinding == nil && stmt.Condition != nil && thenReachable {
		a.recordConditionFact(stmt.Condition, ConditionFactBranchTrue, stmt.Token)
	}
	resultBinding, resultTrueState, resultStateKnown := ifResultStateTest(stmt)
	if stmt.OptionBinding != nil {
		resultStateKnown = resultStateKnown && optionBindingValid
	} else {
		conditionType, typed := a.ResolvedTypeOf(stmt.Condition)
		resultStateKnown = resultStateKnown && typed && conditionType.Kind == BoolType
	}
	if resultStateKnown && thenReachable {
		a.recordResultStateFact(resultBinding, resultTrueState, statementToken(stmt))
	}
	var thenBranch branchAnalysis
	if optionBindingValid {
		thenBranch = a.analyzeOptionBindingBranchWithCallGraphReachability(stmt, optionBinding, thenReachable)
	} else if availabilityTest != nil {
		thenBranch = a.analyzeAvailabilityBranchWithCallGraphReachability(stmt.Consequence, *availabilityTest, !availabilityTest.Negated, thenReachable)
	} else if stateTest != nil {
		thenBranch = a.analyzeStateTestBranch(stmt.Consequence, *stateTest, true, thenReachable)
	} else {
		thenBranch = a.analyzeBranchBlockWithCallGraphReachability(stmt.Consequence, thenReachable)
	}
	a.activeConditionFacts = a.activeConditionFacts[:refinementCount]
	if !thenReachable {
		thenBranch.continues = false
	}
	if stmt.Alternative != nil {
		var elseBranch branchAnalysis
		elseRefinementCount := len(a.activeConditionFacts)
		if stmt.OptionBinding == nil && elseReachable {
			a.recordPathConditionFact(stmt.Condition, false)
		}
		if resultStateKnown {
			a.recordResultStateFact(resultBinding, oppositeResultState(resultTrueState), statementToken(stmt))
		}
		if availabilityTest != nil {
			elseBranch = a.analyzeAvailabilityBranchWithCallGraphReachability(stmt.Alternative, *availabilityTest, availabilityTest.Negated, elseReachable)
		} else if stateTest != nil {
			elseBranch = a.analyzeStateTestBranch(stmt.Alternative, *stateTest, false, elseReachable)
		} else {
			elseBranch = a.analyzeBranchBlockWithCallGraphReachability(stmt.Alternative, elseReachable)
		}
		a.activeConditionFacts = a.activeConditionFacts[:elseRefinementCount]
		if !elseReachable {
			elseBranch.continues = false
		}
		if stmt.OptionBinding == nil && thenBranch.continues != elseBranch.continues {
			// Only one branch reaches the code after the if, so its
			// condition value holds there until the enclosing block ends.
			a.recordPathConditionFact(stmt.Condition, thenBranch.continues)
		}
		if resultStateKnown && thenBranch.continues != elseBranch.continues {
			// Only one branch reaches the code after the if, so its state
			// holds there until the enclosing block ends.
			state := resultTrueState
			if elseBranch.continues {
				state = oppositeResultState(resultTrueState)
			}
			a.recordResultStateFact(resultBinding, state, statementToken(stmt))
		}
		a.recordResolvedIfFlow(stmt, thenReachable, elseReachable, thenBranch, elseBranch)
		a.assigned = mergeContinuingAssigned(before, thenBranch, elseBranch)
		a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, thenBranch, elseBranch)
		a.closedResources = mergeContinuingClosedResources(beforeClosedResources, thenBranch, elseBranch)
		a.borrows = mergeContinuingBorrows(beforeBorrows, thenBranch, elseBranch)
		a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, thenBranch, elseBranch)
		a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, thenBranch, elseBranch)
		return
	}

	fallthroughBranch := branchAnalysis{
		assigned:           before,
		moved:              beforeMoved,
		moveReasons:        beforeMoveReasons,
		closedResources:    beforeClosedResources,
		borrows:            beforeBorrows,
		localRefContainers: beforeLocalRefContainers,
		arenaGenerations:   beforeArenaGenerations,
		continues:          elseReachable,
	}
	if availabilityTest != nil {
		fallthroughBranch = a.refinedAvailabilityFallthrough(fallthroughBranch, *availabilityTest, availabilityTest.Negated)
		fallthroughBranch.continues = elseReachable
	}
	if stateTest != nil {
		if binding, refined := stateTestRefinement(*stateTest, false); refined {
			fallthroughBranch.assigned = copyAssigned(fallthroughBranch.assigned)
			fallthroughBranch.assigned[binding] = true
		}
	}
	if stmt.OptionBinding == nil && !thenBranch.continues && elseReachable {
		a.recordPathConditionFact(stmt.Condition, false)
	}
	if resultStateKnown && !thenBranch.continues && elseReachable {
		// The true branch exits, so the code after the if sees the opposite
		// state until the enclosing block ends.
		a.recordResultStateFact(resultBinding, oppositeResultState(resultTrueState), statementToken(stmt))
	}
	a.recordResolvedIfFlow(stmt, thenReachable, elseReachable, thenBranch, fallthroughBranch)
	a.assigned = mergeContinuingAssigned(before, thenBranch, fallthroughBranch)
	a.moved, a.moveReasons = mergeContinuingMoveState(beforeMoved, beforeMoveReasons, thenBranch, fallthroughBranch)
	a.closedResources = mergeContinuingClosedResources(beforeClosedResources, thenBranch, fallthroughBranch)
	a.borrows = mergeContinuingBorrows(beforeBorrows, thenBranch, fallthroughBranch)
	a.localRefContainers = mergeContinuingLocalRefContainers(beforeLocalRefContainers, thenBranch, fallthroughBranch)
	a.arenaGenerations = mergeContinuingArenaGenerations(beforeArenaGenerations, thenBranch, fallthroughBranch)
}
