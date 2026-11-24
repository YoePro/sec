package sema

import "sec/internal/ast"

// rejectUnimplementedExecutionContracts prevents parsed explicit contracts from
// being erased by the old inferred-only callable model before MD-138 semantics
// and compatibility are implemented. Source selection has already run.
// Rules: rules/concurrency/cancellation.md — §24A(5)–(9);
// rules/compiler/compiler_pipeline.md — §32(2)–(4).
func (a *Analyzer) rejectUnimplementedExecutionContracts(program *ast.Program) bool {
	found := false
	ast.Visit(program, func(node any) bool {
		clause, ok := node.(*ast.ExecutionRequirementClause)
		if !ok || clause.Invalid {
			return true
		}
		found = true
		a.reportCapabilityUnavailable(clause.Token,
			"The require clause is parsed and retained. Explicit execution-contract verification and callable compatibility (MD-138) are not implemented yet.",
			"compiler capability unavailable: explicit callable execution requirements")
		return false
	})
	return found
}
