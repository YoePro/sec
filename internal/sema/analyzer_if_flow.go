package sema

import "sec/internal/ast"

// ResolvedIfPathExecution describes whether one condition edge is impossible,
// conditionally selected, or the only selected edge after semantic folding.
type ResolvedIfPathExecution string

const (
	ResolvedIfPathNever       ResolvedIfPathExecution = "never"
	ResolvedIfPathConditional ResolvedIfPathExecution = "conditional"
	ResolvedIfPathAlways      ResolvedIfPathExecution = "always"
)

// ResolvedIfFlow is the compiler-owned branch-flow fact for one analyzed if.
// The false path exists even without an explicit else because it participates
// in definite-assignment, ownership, and termination merging.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §18 "Definite assignment"
//   - rules/control-flow/flowcontrol_if.md — §19 "Terminating branches"
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements"
type ResolvedIfFlow struct {
	TruePathExecution  ResolvedIfPathExecution
	FalsePathExecution ResolvedIfPathExecution
	HasExplicitElse    bool
	HasNoBranchPath    bool
	TruePathContinues  bool
	FalsePathContinues bool
}

// ResolvedIfFlowOf returns the immutable decision recorded by completed
// semantic analysis. It performs no condition or branch inference.
func (a *Analyzer) ResolvedIfFlowOf(stmt *ast.IfStatement) (ResolvedIfFlow, bool) {
	if a == nil || stmt == nil {
		return ResolvedIfFlow{}, false
	}
	flow, ok := a.resolvedIfFlows[stmt]
	return flow, ok
}

// recordResolvedIfFlow publishes the exact path reachability and continuation
// decisions already made by branch analysis.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §§18–20, §27
func (a *Analyzer) recordResolvedIfFlow(stmt *ast.IfStatement, trueReachable bool, falseReachable bool, trueBranch branchAnalysis, falseBranch branchAnalysis) {
	if a == nil || stmt == nil {
		return
	}
	a.resolvedIfFlows[stmt] = ResolvedIfFlow{
		TruePathExecution:  resolvedIfPathExecution(trueReachable, falseReachable),
		FalsePathExecution: resolvedIfPathExecution(falseReachable, trueReachable),
		HasExplicitElse:    stmt.Alternative != nil,
		HasNoBranchPath:    stmt.Alternative == nil && falseReachable,
		TruePathContinues:  trueBranch.continues,
		FalsePathContinues: falseBranch.continues,
	}
}

func resolvedIfPathExecution(reachable bool, otherReachable bool) ResolvedIfPathExecution {
	if !reachable {
		return ResolvedIfPathNever
	}
	if !otherReachable {
		return ResolvedIfPathAlways
	}
	return ResolvedIfPathConditional
}
