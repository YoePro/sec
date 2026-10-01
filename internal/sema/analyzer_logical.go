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

// inferLogicalExpression implements left-to-right short-circuit flow and uses
// the shared compile-time boolean proof to select the RHS execution edge.
// Impossible RHS diagnostics are retained while runtime state, calls, and
// effects remain isolated from the continuing path.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §§4–6 "Boolean operators", "Short-circuit evaluation", and evaluation order
//   - rules/foundations/operators.md — "Short-circuit evaluation" and "Path-sensitive effects"
//   - rules/corrections/applied/correction22-20260823.md — §§1–5 required logical-flow correction
func (a *Analyzer) inferLogicalExpression(expr *ast.InfixExpression, leftType Type) (Type, expressionValue) {
	if leftType.Kind != BoolType {
		a.addErrorAtToken(expr.Token, "operator %s requires bool operands", expr.Operator)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	execution := LogicalRHSConditional
	leftValue, leftKnown := a.constantBooleanValue(expr.Left)
	shortCircuits := leftKnown && (leftValue && expr.Operator == "||" || !leftValue && expr.Operator == "&&")
	rhsRequired := leftKnown && !shortCircuits
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
	refinementCount := len(a.activeConditionFacts)
	if expr.Operator == "&&" && !shortCircuits {
		a.recordConditionFact(expr.Left, ConditionFactLogicalRHSTrue, expr.Token)
	} else if expr.Operator == "||" && !shortCircuits {
		a.recordConditionFact(expr.Left, ConditionFactLogicalRHSFalse, expr.Token)
	}
	rightType, _ := a.inferExpression(expr.Right)
	a.activeConditionFacts = a.activeConditionFacts[:refinementCount]
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
