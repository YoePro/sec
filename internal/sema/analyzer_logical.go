package sema

import "sec/internal/ast"

// LogicalRHSExecution records whether source analysis proved that a logical
// right operand executes. Conditional means it executes only on the operator's
// selected left-value edge.
type LogicalRHSExecution string

const (
	LogicalRHSNever       LogicalRHSExecution = "never"
	LogicalRHSAlways      LogicalRHSExecution = "always"
	LogicalRHSConditional LogicalRHSExecution = "conditional"
)

// ResolvedLogicalFlow is the compiler-owned short-circuit decision for one
// successfully analyzed && or || expression. Later flow and lowering stages
// can consume the selected edge without reconstructing it from AST spelling.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §§4–6
//   - rules/control-flow/flowcontrol_while.md — §5
//   - rules/foundations/operators.md — "Short-circuit evaluation"
type ResolvedLogicalFlow struct {
	Operator            string
	LeftType            Type
	RightType           Type
	ResultType          Type
	RHSExecution        LogicalRHSExecution
	EvaluateRHSWhenLeft bool
	ShortCircuitResult  bool
}

// ResolvedLogicalFlowOf returns an immutable fact recorded by successful Sema.
// It performs no inference and unknown AST nodes do not mutate analyzer state.
func (a *Analyzer) ResolvedLogicalFlowOf(expr *ast.InfixExpression) (ResolvedLogicalFlow, bool) {
	if a == nil || expr == nil {
		return ResolvedLogicalFlow{}, false
	}
	fact, ok := a.resolvedLogicalFlows[expr]
	return fact, ok
}

// inferLogicalExpression implements the short-circuit control-flow rule from
// rules/control-flow/flowcontrol_if.md and rules/foundations/operators.md.
// correction22 requires RHS diagnostics to be retained while impossible or
// conditional execution effects are isolated and merged into the live state.
func (a *Analyzer) inferLogicalExpression(expr *ast.InfixExpression, leftType Type) (Type, expressionValue) {
	if leftType.Kind != BoolType {
		a.addErrorAtToken(expr.Token, "operator %s requires bool operands", expr.Operator)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	execution := LogicalRHSConditional
	shortCircuits := isBoolLiteral(expr.Left, expr.Operator == "||")
	rhsRequired := isBoolLiteral(expr.Left, expr.Operator == "&&") ||
		isBoolLiteral(expr.Left, false) && expr.Operator == "||"
	if shortCircuits {
		execution = LogicalRHSNever
	} else if rhsRequired {
		execution = LogicalRHSAlways
	}

	before := a.currentBranchAnalysisState()
	previousReachable := a.callGraphPathReachable
	if shortCircuits {
		a.callGraphPathReachable = false
	}
	rightType, _ := a.inferExpression(expr.Right)
	a.callGraphPathReachable = previousReachable

	if shortCircuits {
		a.applyBranchAnalysisState(before)
	} else if !rhsRequired {
		rhs := a.currentBranchAnalysisState()
		a.assigned = mergeContinuingAssigned(before.assigned, before, rhs)
		a.moved, a.moveReasons = mergeContinuingMoveState(before.moved, before.moveReasons, before, rhs)
		a.closedResources = mergeContinuingClosedResources(before.closedResources, before, rhs)
		a.borrows = mergeContinuingBorrows(before.borrows, before, rhs)
		a.localRefContainers = mergeContinuingLocalRefContainers(before.localRefContainers, before, rhs)
		a.arenaGenerations = mergeContinuingArenaGenerations(before.arenaGenerations, before, rhs)
	}

	if rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if rightType.Kind != BoolType {
		a.addErrorAtToken(expr.Token, "operator %s requires bool operands", expr.Operator)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	result := Type{Name: "bool", Kind: BoolType}
	a.resolvedLogicalFlows[expr] = ResolvedLogicalFlow{
		Operator:            expr.Operator,
		LeftType:            leftType,
		RightType:           rightType,
		ResultType:          result,
		RHSExecution:        execution,
		EvaluateRHSWhenLeft: expr.Operator == "&&",
		ShortCircuitResult:  expr.Operator == "||",
	}
	return result, expressionValue{Display: expr.String()}
}
