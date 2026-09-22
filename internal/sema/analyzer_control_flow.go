package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// diagnoseConstantConditionUnreachableBlock reports the first statement in a
// block excluded by a literal boolean condition. Empty blocks remain valid,
// and ordinary block analysis still runs for independent source diagnostics.
//
// Rules:
//   - rules/tooling/diagnostics.txt — "Unreachable branches"
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
func (a *Analyzer) diagnoseConstantConditionUnreachableBlock(block *ast.BlockStatement, region string) {
	if block == nil || len(block.Statements) == 0 {
		return
	}
	a.addErrorAtTokenWithMetadata(
		statementToken(block.Statements[0]),
		diagnostics.UnreachableStatement,
		fmt.Sprintf("The constant condition makes this %s impossible. Remove it or change the condition.", region),
		"unreachable statement",
	)
}
