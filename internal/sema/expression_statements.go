package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// analyzeExpressionStatement checks standalone result obligations before
// recording optional guidance for a legal implicit ordinary call-result discard.
// Rules: rules/control-flow/discard.md — §§10–13,37–38;
// rules/errors/errorhandling.md — must-use Result handling.
func (a *Analyzer) analyzeExpressionStatement(stmt *ast.ExpressionStatement) {
	if tryExpr, ok := stmt.Expression.(*ast.TryExpression); ok {
		if a.statementTries == nil {
			a.statementTries = map[*ast.TryExpression]bool{}
		}
		a.statementTries[tryExpr] = true
	}
	errorsBefore := len(a.errors)
	exprType, _ := a.inferExpression(stmt.Expression)
	if _, ok := stmt.Expression.(*ast.SpawnExpression); ok && exprType.Kind != InvalidType {
		a.addErrorAtToken(stmt.Token, "spawned task result must be owned, awaited, joined or detached")
		return
	}
	if exprType.Kind == InvalidType || exprType.Kind == VoidType || !isCallLikeExpression(stmt.Expression) {
		return
	}
	if !isMustUseType(exprType) {
		if len(a.errors) == errorsBefore {
			a.recordImplicitDiscard(stmt, exprType)
		}
		return
	}
	if a.inDeferBlock && exprType.Kind == ResultType {
		a.addErrorAtTokenWithMetadata(
			expressionToken(stmt.Expression),
			diagnostics.UnhandledMustUseResult,
			"use try, match, binding, return, or explicit discard",
			"unhandled Result inside defer; handle it or discard it explicitly",
		)
		return
	}
	a.addErrorAtTokenWithMetadata(
		expressionToken(stmt.Expression),
		diagnostics.UnhandledMustUseResult,
		"use try, match, binding, return, or explicit discard",
		"result of %s has type %s and must be handled explicitly",
		expressionStatementDisplayName(stmt.Expression),
		typeDisplayName(exprType),
	)
}
