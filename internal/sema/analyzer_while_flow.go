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
	// The condition runs before every iteration, so places it consumes are
	// backedge state, not loop-entry state; keep the pre-condition moves for
	// the backedge fixed point (rules/memory/ownership.md, loop fixed point).
	preConditionMoved := copyMoved(a.moved)
	preConditionReasons := copyAvailabilityStates(a.availabilityStates)
	if stmt.Condition != nil {
		conditionType, _ := a.inferExpression(stmt.Condition)
		if conditionType.Kind != InvalidType && conditionType.Kind != BoolType {
			a.addErrorAtToken(expressionToken(stmt.Condition), "%s", nonBoolConditionMessage(stmt.Condition, "while", conditionType))
		}
		if conditionType.Kind == BoolType {
			constantCondition, constantConditionKnown = a.constantBooleanValue(stmt.Condition)
			if constantConditionKnown && !constantCondition {
				a.diagnoseConstantConditionUnreachableBlock(stmt.Body, "loop body")
			}
		}
	}

	// A pure condition that dominating path facts decide at entry: proven
	// false, the loop runs zero times like a constant-false loop and its body
	// is unreachable; proven true, the zero-iteration exit is impossible.
	entryProvenTrue := false
	if stmt.Condition != nil && !constantConditionKnown {
		if value, known := a.relationalConditionValue(stmt.Condition); known {
			if value {
				entryProvenTrue = true
			} else {
				a.diagnoseRelationallyUnreachableBlock(stmt.Body, false, "loop body")
				constantConditionKnown, constantCondition = true, false
			}
		}
	}

	// Whether the condition holds on first entry, from the constants before
	// the loop: a proven-true first test removes the zero-iteration exit.
	firstEntryHolds, firstEntryKnown := false, false
	if stmt.Condition != nil && !constantConditionKnown {
		firstEntryHolds, firstEntryKnown = a.firstIterationCondition(stmt.Condition, nil)
		if entryProvenTrue {
			firstEntryHolds, firstEntryKnown = true, true
		}
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.availabilityStates
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousLoopDepth := a.loopDepth
	frame := a.pushLoopBreakFrame()

	a.symbols = previousSymbols.child()
	a.constInts = copyConstInts(previousConstInts)
	endFirstIteration := a.forgetLoopAssignedConstants(stmt.Body, !constantConditionKnown || constantCondition, stmt.Condition)
	defer endFirstIteration()
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.availabilityStates = copyAvailabilityStates(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.loopDepth++
	defer a.enterCleanupLoop()()
	defer a.enterLoopBody(stmt.Token)()
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
		bodyFacts := len(a.activeConditionFacts)
		a.enterLoopBodyFacts()
		if loopBodyReachable && !constantConditionKnown {
			// The condition is re-evaluated on every iteration, so it holds
			// at each body entry whatever the body mutates.
			a.recordPathConditionFact(stmt.Condition, true)
		}
		a.analyzeBlockStatements(stmt.Body)
		a.activeConditionFacts = a.activeConditionFacts[:bodyFacts]
	}
	endFirstIteration()
	if stmt.Body != nil {
	}
	hasReachableBreak := a.blockHasReachableBreakToCurrentLoop(stmt.Body)
	// A loop whose first test is proven true and whose body never reaches
	// the next test (no fall-through, no continue) exits only by break.
	reachesNextTest := stmt.Body == nil || a.blockCanFallThrough(stmt.Body) || len(a.loopBreakFrames[frame].continues) > 0
	entersOnceOnly := firstEntryKnown && firstEntryHolds && !reachesNextTest
	a.recordResolvedWhileFlow(stmt, constantConditionKnown, constantCondition, hasReachableBreak, entersOnceOnly)
	if constantConditionKnown && !constantCondition {
		a.callGraphPathReachable = previousCallGraphPathReachable
		a.popLoopBreakFrame(frame)
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.availabilityStates = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.loopDepth = previousLoopDepth
		return
	}

	loopConstInts := a.constInts
	loopAssigned := a.assigned
	loopMoved := a.moved
	loopMoveReasons := a.availabilityStates
	loopClosedResources := a.closedResources
	loopBorrows := a.borrows
	loopLocalRefContainers := a.localRefContainers
	loopArenaGenerations := a.arenaGenerations
	frameState := a.loopBreakFrames[frame]
	bodyFallsThrough := a.blockCanFallThrough(stmt.Body)
	headerMoved, headerReasons := loopBackedgeMoveState(preConditionMoved, preConditionReasons, loopMoved, loopMoveReasons, frameState, bodyFallsThrough)
	headerClosedResources := loopBackedgeClosedResourceState(iterationEntry.closedResources, loopClosedResources, frameState, bodyFallsThrough)
	headerBorrows := loopBackedgeBorrowState(iterationEntry.borrows, loopBorrows, frameState, bodyFallsThrough)
	headerLocalRefContainers := loopBackedgeReferenceState(iterationEntry.localRefContainers, loopLocalRefContainers, frameState, bodyFallsThrough)
	headerArenaGenerations := loopBackedgeArenaGenerationState(iterationEntry.arenaGenerations, loopArenaGenerations, frameState, bodyFallsThrough)
	backedgeEntry := iterationEntry
	backedgeEntry.moved = preConditionMoved
	a.checkLoopBackedgeFixedPoint(stmt.Condition, stmt.Body, backedgeEntry, headerMoved, headerReasons, headerClosedResources, headerBorrows, headerLocalRefContainers, headerArenaGenerations, loopHeaderReferenceOrigins(iterationEntry.symbols, a.symbols, bodyFallsThrough, frameState))
	a.callGraphPathReachable = previousCallGraphPathReachable
	breakFrame := a.popLoopBreakFrame(frame)
	loopSymbols := a.symbols
	a.symbols = previousSymbols
	a.mergeLoopExitReferenceOrigins(loopSymbols, breakFrame)
	a.constInts = previousConstInts
	for name, previousValue := range previousConstInts {
		currentValue, exists := loopConstInts[name]
		if !exists || currentValue.Cmp(previousValue) != 0 {
			delete(a.constInts, name)
		}
	}
	conditionAlwaysTrue := constantConditionKnown && constantCondition
	// Definite assignment after the loop is the intersection over every
	// reachable exit: the zero-iteration condition-false exit (absent when
	// the first test is proven true), the condition-false exit after an
	// iteration (body fall-through and continue edges), and each reachable
	// break (flowcontrol_while.md § 28).
	assignedExits := []map[string]bool{}
	refinedBinding := ""
	if hasStateTest {
		// rules/declarations/unions.md — §8.3: a false `is empty` test proves
		// the binding initialized on every condition-false exit.
		if binding, refined := stateTestRefinement(stateTest, false); refined {
			refinedBinding = binding
		}
	}
	conditionFalseExit := func(assigned map[string]bool) {
		if refinedBinding != "" {
			assigned = copyAssigned(assigned)
			assigned[refinedBinding] = true
		}
		assignedExits = append(assignedExits, assigned)
	}
	if !conditionAlwaysTrue && stmt.Condition != nil {
		if !firstEntryKnown || !firstEntryHolds {
			conditionFalseExit(previousAssigned)
		}
		if bodyFallsThrough {
			conditionFalseExit(loopAssigned)
		}
		for _, edge := range breakFrame.continues {
			conditionFalseExit(edge.assigned)
		}
	}
	if hasReachableBreak {
		assignedExits = append(assignedExits, breakFrame.breakAssigned()...)
	}
	a.assigned = intersectExitAssigned(previousAssigned, refinedBinding, assignedExits)
	a.moved, a.availabilityStates = mergeLoopMoveState(previousMoved, previousMoveReasons, loopMoved, loopMoveReasons, breakFrame)
	a.closedResources = mergeLoopClosedResourceState(previousClosedResources, loopClosedResources, breakFrame, bodyFallsThrough, conditionAlwaysTrue)
	a.borrows = mergeLoopBorrowState(previousBorrows, loopBorrows, breakFrame)
	a.localRefContainers = mergeLoopReferenceState(previousLocalRefContainers, loopLocalRefContainers, breakFrame)
	a.arenaGenerations = mergeLoopArenaGenerations(previousArenaGenerations, loopArenaGenerations, breakFrame.breakArenaGenerations())
	a.loopDepth = previousLoopDepth
	// A variant proven on every exit edge (each reachable break, and the
	// condition-false exit unless the condition is always true) holds after
	// the loop (flowcontrol_while.md § 28; MD-042).
	exits := make([]knownVariantSet, 0, len(breakFrame.breaks)+1)
	if hasReachableBreak {
		for _, edge := range breakFrame.breaks {
			exits = append(exits, edge.knownVariants)
		}
	}
	if !conditionAlwaysTrue && stmt.Condition != nil {
		exits = append(exits, a.knownVariantsAfterNegation(stmt.Condition))
	}
	a.assertKnownVariants(intersectKnownVariants(exits))
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
func (a *Analyzer) recordResolvedWhileFlow(stmt *ast.WhileStatement, conditionKnown bool, conditionValue bool, hasReachableBreak bool, entersOnceOnly bool) {
	if a == nil || stmt == nil {
		return
	}
	a.resolvedWhileFlows[stmt] = ResolvedWhileFlow{
		ConditionKnown:     conditionKnown,
		ConditionValue:     conditionValue,
		HasReachableBreak:  hasReachableBreak,
		ContinuesAfterLoop: (!conditionKnown || !conditionValue || hasReachableBreak) && (!entersOnceOnly || hasReachableBreak),
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

// nonBoolConditionMessage reports a non-bool if or while condition. A
// bodyless try condition uses the rulebooks' focused wording,
// "try expression in if condition must produce bool, got T".
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §11 "`try` in a condition", diagnostics
//   - rules/control-flow/flowcontrol_while.md — §7 "`try` in a condition", diagnostics
func nonBoolConditionMessage(condition ast.Expression, construct string, got Type) string {
	if try, ok := condition.(*ast.TryExpression); ok && len(try.Handlers) == 0 {
		return "try expression in " + construct + " condition must produce bool, got " + typeDisplayName(got)
	}
	return construct + " condition must be bool, got " + typeDisplayName(got)
}

// intersectExitAssigned keeps a binding visible before the loop (or the
// state-test binding) definitely assigned only when every exit assigns it. With
// no reachable exit, the code after the loop is unreachable and the state
// before the loop is retained.
func intersectExitAssigned(before map[string]bool, refinedBinding string, exits []map[string]bool) map[string]bool {
	if len(exits) == 0 {
		return before
	}
	merged := copyAssigned(before)
	names := make([]string, 0, len(before)+1)
	for name := range before {
		names = append(names, name)
	}
	if refinedBinding != "" {
		names = append(names, refinedBinding)
	}
	for _, name := range names {
		assigned := true
		for _, exit := range exits {
			if !exit[name] {
				assigned = false
				break
			}
		}
		merged[name] = assigned
	}
	return merged
}
