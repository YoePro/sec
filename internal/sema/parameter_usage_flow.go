package sema

import "sec/internal/ast"

// walkBlock records demand only until the current block can no longer
// continue. The completed Sema flow facts remain authoritative for constructs
// whose reachability cannot be recovered safely from syntax alone.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Unreachable paths"
//   - rules/analysis/parameter_usage_analysis.md — "Control-flow joins"
func (b *parameterUsageBuilder) walkBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		b.walkStatement(statement)
		if !b.statementCanFallThrough(statement) {
			return
		}
	}
}

// statementCanFallThrough consumes compiler-owned if-flow decisions before
// falling back to the shared Sema control-flow query. This preserves dynamic
// branches while excluding paths eliminated by availability and other
// canonical compile-time reasoning.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Unreachable paths"
//   - rules/control-flow/flowcontrol_if.md — §19 "Terminating branches"
//   - rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements"
func (b *parameterUsageBuilder) statementCanFallThrough(statement ast.Statement) bool {
	if branch, ok := statement.(*ast.IfStatement); ok {
		if flow, resolved := b.analyzer.ResolvedIfFlowOf(branch); resolved {
			trueContinues := flow.TruePathExecution != ResolvedIfPathNever && flow.TruePathContinues
			falseContinues := flow.FalsePathExecution != ResolvedIfPathNever && flow.FalsePathContinues
			return trueContinues || falseContinues
		}
	}
	return b.analyzer.statementCanFallThrough(statement)
}

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
