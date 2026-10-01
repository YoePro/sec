package sema

import "sec/internal/ast"

// analyzeWhileStatement validates while conditions and computes the loop's
// reachable fixed-point and continuation state. Proven-false bodies remain
// statically checked, but only their zero-iteration exit reaches later code.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§19–20 "Constant conditions" and "Non-continuing while true"
//   - rules/control-flow/flowcontrol_while.md — §§28–29 "Sema and flow-analysis requirements" and "CFG requirements"
//   - rules/analysis/call_graph.md — "Path sensitivity"
//   - rules/analysis/effect_analysis.md — "Path sensitivity"
func (a *Analyzer) analyzeWhileStatement(stmt *ast.WhileStatement) {
	constantCondition := false
	constantConditionKnown := false
	if stmt.Condition != nil {
		conditionType, _ := a.inferExpression(stmt.Condition)
		if conditionType.Kind != InvalidType && conditionType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(stmt.Condition), "while condition must be bool, got %s", typeDisplayName(conditionType))
		}
		if conditionType.Kind == BoolType {
			constantCondition, constantConditionKnown = a.constantBooleanValue(stmt.Condition)
			if constantConditionKnown && !constantCondition {
				a.diagnoseConstantConditionUnreachableBlock(stmt.Body, "loop body")
			}
		}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousLoopDepth := a.loopDepth
	frame := a.pushLoopBreakFrame()

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.loopDepth++
	stateTest, hasStateTest := a.whileConditionStateTest(stmt)
	if hasStateTest {
		if binding, refined := stateTestRefinement(stateTest, true); refined {
			a.assigned[binding] = true
		}
	}
	iterationEntry := a.captureLoopIterationAnalysisState()
	previousCallGraphPathReachable := a.callGraphPathReachable
	loopBodyReachable := !constantConditionKnown || constantCondition
	a.callGraphPathReachable = previousCallGraphPathReachable && loopBodyReachable

	if stmt.Body != nil {
		a.analyzeBlockStatements(stmt.Body)
	}
	hasReachableBreak := a.blockHasReachableBreakToCurrentLoop(stmt.Body)
	a.recordResolvedWhileFlow(stmt, constantConditionKnown, constantCondition, hasReachableBreak)
	if constantConditionKnown && !constantCondition {
		a.callGraphPathReachable = previousCallGraphPathReachable
		a.popLoopBreakFrame(frame)
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.loopDepth = previousLoopDepth
		return
	}

	loopConstInts := a.constInts
	loopMoved := a.moved
	loopMoveReasons := a.moveReasons
	loopClosedResources := a.closedResources
	loopBorrows := a.borrows
	loopLocalRefContainers := a.localRefContainers
	loopArenaGenerations := a.arenaGenerations
	frameState := a.loopBreakFrames[frame]
	bodyFallsThrough := a.blockCanFallThrough(stmt.Body)
	headerMoved, headerReasons := loopBackedgeMoveState(iterationEntry.moved, iterationEntry.moveReasons, loopMoved, loopMoveReasons, frameState, bodyFallsThrough)
	headerClosedResources := loopBackedgeClosedResourceState(iterationEntry.closedResources, loopClosedResources, frameState, bodyFallsThrough)
	headerBorrows := loopBackedgeBorrowState(iterationEntry.borrows, loopBorrows, frameState, bodyFallsThrough)
	headerLocalRefContainers := loopBackedgeReferenceState(iterationEntry.localRefContainers, loopLocalRefContainers, frameState, bodyFallsThrough)
	headerArenaGenerations := loopBackedgeArenaGenerationState(iterationEntry.arenaGenerations, loopArenaGenerations, frameState, bodyFallsThrough)
	a.checkLoopBackedgeFixedPoint(stmt.Condition, stmt.Body, iterationEntry, headerMoved, headerReasons, headerClosedResources, headerBorrows, headerLocalRefContainers, headerArenaGenerations)
	a.callGraphPathReachable = previousCallGraphPathReachable
	breakFrame := a.popLoopBreakFrame(frame)
	a.symbols = previousSymbols
	a.constInts = previousConstInts
	for name, previousValue := range previousConstInts {
		currentValue, exists := loopConstInts[name]
		if !exists || currentValue.Cmp(previousValue) != 0 {
			delete(a.constInts, name)
		}
	}
	conditionAlwaysTrue := constantConditionKnown && constantCondition
	if conditionAlwaysTrue && hasReachableBreak && len(breakFrame.assignments) > 0 {
		a.assigned = mergeBreakAssigned(previousAssigned, breakFrame.assignments)
	} else {
		a.assigned = previousAssigned
	}
	// rules/declarations/unions.md — §8.3: without a reachable break the loop
	// exits only through its condition becoming false, so a false `is empty`
	// test proves the binding initialized after the loop.
	if hasStateTest && !hasReachableBreak {
		if binding, refined := stateTestRefinement(stateTest, false); refined {
			a.assigned = copyAssigned(a.assigned)
			a.assigned[binding] = true
		}
	}
	a.moved, a.moveReasons = mergeLoopMoveState(previousMoved, previousMoveReasons, loopMoved, loopMoveReasons, breakFrame)
	a.closedResources = mergeLoopClosedResourceState(previousClosedResources, loopClosedResources, breakFrame, bodyFallsThrough, conditionAlwaysTrue)
	a.borrows = mergeLoopBorrowState(previousBorrows, loopBorrows, breakFrame)
	a.localRefContainers = mergeLoopReferenceState(previousLocalRefContainers, loopLocalRefContainers, breakFrame)
	a.arenaGenerations = mergeLoopArenaGenerations(previousArenaGenerations, loopArenaGenerations, breakFrame.arenaGenerations)
	a.loopDepth = previousLoopDepth
}

// ResolvedWhileFlow is the compiler-owned condition and exit fact for one
// analyzed while statement. Unknown runtime conditions retain the normal
// zero-iteration continuation; a compile-time true condition continues only
// through a reachable break targeting this loop.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§19–20 "Constant conditions" and "Non-continuing while true"
//   - rules/control-flow/flowcontrol_while.md — §28 "Sema and flow-analysis requirements"
type ResolvedWhileFlow struct {
	ConditionKnown     bool
	ConditionValue     bool
	HasReachableBreak  bool
	ContinuesAfterLoop bool
}

// ResolvedWhileFlowOf returns the immutable decision recorded by completed
// semantic analysis without re-evaluating the source condition.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §28 "Sema and flow-analysis requirements"
func (a *Analyzer) ResolvedWhileFlowOf(stmt *ast.WhileStatement) (ResolvedWhileFlow, bool) {
	if a == nil || stmt == nil {
		return ResolvedWhileFlow{}, false
	}
	flow, ok := a.resolvedWhileFlows[stmt]
	return flow, ok
}

// recordResolvedWhileFlow publishes the condition proof and current-loop exit
// decision already established during body analysis.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§19–20, §28
func (a *Analyzer) recordResolvedWhileFlow(stmt *ast.WhileStatement, conditionKnown bool, conditionValue bool, hasReachableBreak bool) {
	if a == nil || stmt == nil {
		return
	}
	a.resolvedWhileFlows[stmt] = ResolvedWhileFlow{
		ConditionKnown:     conditionKnown,
		ConditionValue:     conditionValue,
		HasReachableBreak:  hasReachableBreak,
		ContinuesAfterLoop: !conditionKnown || !conditionValue || hasReachableBreak,
	}
}

// blockHasReachableBreakToCurrentLoop follows non-loop control-flow containers
// and consumes resolved if-path facts so an impossible branch cannot create a
// false loop exit. Nested loops own their own breaks.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §20 "Non-continuing while true"
//   - rules/control-flow/flowcontrol_while.md — §28 "Sema and flow-analysis requirements"
//   - rules/control-flow/flowcontrol_for.md — nearest-loop break ownership
func (a *Analyzer) blockHasReachableBreakToCurrentLoop(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, statement := range block.Statements {
		if a.statementHasReachableBreakToCurrentLoop(statement) {
			return true
		}
	}
	return false
}

// statementHasReachableBreakToCurrentLoop applies current-loop break ownership
// to one already analyzed statement and uses child control-flow facts where
// available.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§20 and 28
//   - rules/control-flow/flowcontrol_for.md — nearest-loop break ownership
func (a *Analyzer) statementHasReachableBreakToCurrentLoop(statement ast.Statement) bool {
	switch statement := statement.(type) {
	case *ast.BreakStatement:
		return true
	case *ast.IfStatement:
		if flow, ok := a.resolvedIfFlows[statement]; ok {
			return flow.TruePathExecution != ResolvedIfPathNever && a.blockHasReachableBreakToCurrentLoop(statement.Consequence) ||
				flow.FalsePathExecution != ResolvedIfPathNever && a.blockHasReachableBreakToCurrentLoop(statement.Alternative)
		}
		return a.blockHasReachableBreakToCurrentLoop(statement.Consequence) || a.blockHasReachableBreakToCurrentLoop(statement.Alternative)
	case *ast.ForStatement, *ast.WhileStatement:
		return false
	case *ast.SwitchStatement:
		for _, clause := range statement.Cases {
			if clause != nil && a.blockHasReachableBreakToCurrentLoop(clause.Body) {
				return true
			}
		}
		return statement.Default != nil && a.blockHasReachableBreakToCurrentLoop(statement.Default.Body)
	case *ast.SelectStatement:
		for _, branch := range statement.Branches {
			if branch != nil && a.blockHasReachableBreakToCurrentLoop(branch.Body) {
				return true
			}
		}
		return false
	case *ast.UnsafeStatement:
		return a.blockHasReachableBreakToCurrentLoop(statement.Body)
	default:
		return false
	}
}

// whileConditionStateTest returns the resolved union state test that forms a
// while condition, if any.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §8 "`is` state tests"
//   - rules/declarations/unions.md — §8.3 "Data-flow refinement"
func (a *Analyzer) whileConditionStateTest(stmt *ast.WhileStatement) (ResolvedStateTest, bool) {
	test, ok := stmt.Condition.(*ast.StateTestExpression)
	if !ok {
		return ResolvedStateTest{}, false
	}
	fact, ok := a.resolvedStateTests[test]
	return fact, ok
}
