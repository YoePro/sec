package sema

import "sec/internal/ast"

// blockDefinitelyReturns derives a block exit from analyzer-owned statement
// flow, including normalized match coverage instead of pattern spellings.
//
// Rules: rules/control-flow/flowcontrol_match.md — §§5,23–24;
// rules/control-flow/flowcontrol_if.md — §27;
// rules/control-flow/flowcontrol_while.md — §§19–20;
// rules/memory/unsafe.md — §4 (lexical block).
func (a *Analyzer) blockDefinitelyReturns(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}

	for _, stmt := range block.Statements {
		if a.statementDefinitelyReturns(stmt) {
			return true
		}
	}
	return false
}

// statementDefinitelyReturns consumes resolved branch/match/switch/loop
// flow through lexical unsafe blocks, preserving the existing asm return rule.
//
// Rules: rules/control-flow/flowcontrol_match.md — §§5,23–24;
// rules/control-flow/flowcontrol_if.md — §27;
// rules/control-flow/flowcontrol_while.md — §§19–20;
// rules/memory/unsafe.md — §4 (lexical block).
func (a *Analyzer) statementDefinitelyReturns(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.IfStatement:
		if flow, ok := a.resolvedIfFlows[stmt]; ok {
			return !flow.TruePathContinues && !flow.FalsePathContinues
		}
	case *ast.UnsafeStatement:
		if unsafeAsmReturns(stmt) {
			return true
		}
		return a.blockDefinitelyReturns(stmt.Body)
	case *ast.MatchStatement:
		return a.matchStatementDefinitelyReturns(stmt)
	case *ast.SwitchStatement:
		return a.switchStatementDefinitelyReturns(stmt)
	case *ast.WhileStatement:
		if flow, ok := a.resolvedWhileFlows[stmt]; ok {
			return flow.ConditionKnown && flow.ConditionValue && !flow.HasReachableBreak
		}
	}
	return statementDefinitelyReturns(stmt)
}

// matchStatementDefinitelyReturns requires exhaustive resolved coverage and
// a non-continuing flow for every represented arm.
//
// Rules: rules/control-flow/flowcontrol_match.md — §§5,23–24;
// rules/control-flow/flowcontrol_if.md — §27;
// rules/control-flow/flowcontrol_while.md — §§19–20;
// rules/memory/unsafe.md — §4 (lexical block).
func (a *Analyzer) matchStatementDefinitelyReturns(stmt *ast.MatchStatement) bool {
	if stmt == nil || stmt.Match == nil {
		return false
	}
	// rules/control-flow/flowcontrol_match.md; correction23.md: termination
	// consumes the canonical normalized coverage and arm-flow plan. Rebuilding
	// coverage from pattern spellings loses enum aliases and union domains.
	plan, ok := a.resolvedMatchPlans[stmt.Match]
	if !ok {
		plan, ok = a.matchTerminationPlans[stmt.Match]
	}
	if !ok || !plan.Exhaustive || len(plan.Arms) == 0 {
		return false
	}
	for _, arm := range plan.Arms {
		switch arm.Flow {
		case MatchArmReturns, MatchArmTerminates, MatchArmLoopControl:
		default:
			return false
		}
	}
	return true
}

// matchArmDefinitelyReturns classifies explicit arm returns and block bodies
// with the same analyzer-aware flow as the enclosing function.
//
// Rules: rules/control-flow/flowcontrol_match.md — §§5,23–24;
// rules/control-flow/flowcontrol_if.md — §27;
// rules/control-flow/flowcontrol_while.md — §§19–20;
// rules/memory/unsafe.md — §4 (lexical block).
func (a *Analyzer) matchArmDefinitelyReturns(arm *ast.MatchArm) bool {
	if arm.ReturnBody != nil {
		return true
	}
	return a.blockDefinitelyReturns(arm.BlockBody)
}

// blockCanFallThrough recognizes nested match and resolved conditional exits
// while retaining an ordinary continuation when every statement can continue.
//
// Rules: rules/control-flow/flowcontrol_match.md — §§5,23–24;
// rules/control-flow/flowcontrol_if.md — §27;
// rules/control-flow/flowcontrol_while.md — §§19–20;
// rules/memory/unsafe.md — §4 (lexical block).
func (a *Analyzer) blockCanFallThrough(block *ast.BlockStatement) bool {
	if block == nil {
		return true
	}
	for _, stmt := range block.Statements {
		if !a.statementCanFallThrough(stmt) {
			return false
		}
	}
	return true
}

// statementCanFallThrough queries the resolved continuation of structured
// statements without losing canonical match flow through nested blocks.
//
// Rules: rules/control-flow/flowcontrol_match.md — §§5,23–24;
// rules/control-flow/flowcontrol_if.md — §27;
// rules/control-flow/flowcontrol_while.md — §§19–20;
// rules/memory/unsafe.md — §4 (lexical block).
func (a *Analyzer) statementCanFallThrough(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.MatchStatement:
		return !a.matchStatementDefinitelyReturns(stmt)
	case *ast.IfStatement:
		// The resolved flow already accounts for branches that folded
		// constants or dominating relations prove impossible.
		if flow, ok := a.resolvedIfFlows[stmt]; ok {
			return flow.TruePathContinues || flow.FalsePathContinues
		}
		if isBoolLiteral(stmt.Condition, true) {
			return a.blockCanFallThrough(stmt.Consequence)
		}
		if isBoolLiteral(stmt.Condition, false) {
			return a.blockCanFallThrough(stmt.Alternative)
		}
		if stmt.Alternative == nil {
			return true
		}
		return a.blockCanFallThrough(stmt.Consequence) || a.blockCanFallThrough(stmt.Alternative)
	case *ast.WhileStatement:
		if flow, ok := a.resolvedWhileFlows[stmt]; ok {
			return flow.ContinuesAfterLoop
		}
		return statementCanFallThrough(stmt)
	case *ast.UnsafeStatement:
		return a.blockCanFallThrough(stmt.Body)
	default:
		return statementCanFallThrough(stmt)
	}
}
