package sema

import "sec/internal/ast"

// walkIfStatement records the condition demand and excludes only branches
// that completed semantic analysis has proven impossible. Missing flow facts
// retain the conservative pre-existing behavior and visit both branches.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Unreachable paths"
//   - rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements"
func (b *parameterUsageBuilder) walkIfStatement(statement *ast.IfStatement) {
	if statement == nil {
		return
	}
	b.walkExpression(statement.Condition)

	flow, resolved := b.analyzer.ResolvedIfFlowOf(statement)
	if !resolved {
		b.walkBlock(statement.Consequence)
		b.walkBlock(statement.Alternative)
		return
	}
	if flow.TruePathExecution != ResolvedIfPathNever {
		b.walkBlock(statement.Consequence)
	}
	if flow.FalsePathExecution != ResolvedIfPathNever {
		b.walkBlock(statement.Alternative)
	}
}
