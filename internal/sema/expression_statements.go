package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// analyzeExpressionStatement checks standalone result obligations before
// recording optional guidance for a legal implicit ordinary call-result discard.
// It returns the inferred type so statement match arms reuse the same analysis
// without treating their result as an owning branch-value transfer.
// Rules: rules/control-flow/discard.md — §§10–13,37–38;
// rules/errors/errorhandling.md — must-use Result handling;
// rules/control-flow/flowcontrol_match.md — §19(2)-(6).
func (a *Analyzer) analyzeExpressionStatement(stmt *ast.ExpressionStatement) Type {
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
		return exprType
	}
	if exprType.Kind == InvalidType || exprType.Kind == VoidType {
		return exprType
	}
	if !isMustUseType(exprType) {
		if !a.isDiscardable(exprType) {
			a.addErrorAtTokenWithMetadata(expressionToken(stmt.Expression), diagnostics.NonDiscardableValue,
				"Bind the result and resolve its lifecycle obligation instead of ignoring it.",
				"result of %s has type %s and cannot be implicitly discarded", expressionStatementDisplayName(stmt.Expression), typeDisplayName(exprType))
			return exprType
		}
		if isCallLikeExpression(stmt.Expression) && len(a.errors) == errorsBefore {
			a.recordImplicitDiscard(stmt, exprType)
		}
		a.endTemporaryArenaBacking(stmt.Expression)
		return exprType
	}
	if a.inDeferBlock && exprType.Kind == ResultType {
		a.addErrorAtTokenWithMetadata(
			expressionToken(stmt.Expression),
			diagnostics.UnhandledMustUseResult,
			"use try, match, binding, return, or explicit discard",
			"unhandled Result inside defer; handle it or discard it explicitly",
		)
		return exprType
	}
	a.addErrorAtTokenWithMetadata(
		expressionToken(stmt.Expression),
		diagnostics.UnhandledMustUseResult,
		"use try, match, binding, return, or explicit discard",
		"result of %s has type %s and must be handled explicitly",
		expressionStatementDisplayName(stmt.Expression),
		typeDisplayName(exprType),
	)
	return exprType
}
