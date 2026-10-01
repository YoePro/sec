package sema

import "sec/internal/ast"

// analyzeTryHandlerGuard checks a handler's `where` condition in the arm scope
// where the pattern binding is already visible. The guard runs before the
// handler is selected, so it may inspect but must not consume the binding.
//
// Rules:
//   - rules/errors/errorhandling.md — §19 "Guards", §20 "Handler ownership and guards"
//   - rules/errors/errorhandling.md — §22 "Handler failures are outside the protected set"
func (a *Analyzer) analyzeTryHandlerGuard(handler *ast.TryHandler, bindingName string) {
	guardType, _ := a.inferExpression(handler.Guard)
	if guardType.Kind != InvalidType && guardType.Kind != BoolType {
		a.addErrorAtToken(expressionToken(handler.Guard), "try handler guard must be bool, got %s", typeDisplayName(guardType))
	}
	if bindingName == "" {
		return
	}
	if token, moved := a.moved[bindingName]; moved {
		a.addErrorAtToken(token, "try handler guard must not consume %s before the handler is selected; inspect or borrow it in the guard and move it in the handler body", bindingName)
	}
}

// tryHandlerBlockValue returns the final expression statement of a handler
// block, which provides the recovery value in result position.
func tryHandlerBlockValue(block *ast.BlockStatement) (ast.Expression, bool) {
	if block == nil || len(block.Statements) == 0 {
		return nil, false
	}
	last := block.Statements[len(block.Statements)-1]
	statement, ok := last.(*ast.ExpressionStatement)
	if !ok || statement.Expression == nil {
		return nil, false
	}
	return statement.Expression, true
}

// analyzeTryHandlerBlockValue analyzes a handler block whose final expression
// is its recovery value: the preceding statements run in the handler scope and
// the final expression must be assignable to the try success type.
//
// Rules:
//   - rules/errors/errorhandling.md — §21 "Recovery values", §21.1 "Block handler result position"
func (a *Analyzer) analyzeTryHandlerBlockValue(handler *ast.TryHandler, value ast.Expression, successType Type) ResolvedTryHandlerFlow {
	block := handler.BlockBody
	prefix := &ast.BlockStatement{Token: block.Token, Tokens: block.Tokens, Statements: block.Statements[:len(block.Statements)-1]}
	a.analyzeBlockStatements(prefix)
	if len(prefix.Statements) > 0 && !blockCanFallThrough(prefix) {
		a.addErrorAtToken(expressionToken(value), "unreachable recovery value; the handler block already leaves control flow")
		if blockDefinitelyReturns(prefix) {
			return TryHandlerReturns
		}
		return TryHandlerTerminates
	}
	valueType, _ := a.inferExpressionWithExpected(value, successType)
	if valueType.Kind == InvalidType {
		return TryHandlerInvalidFlow
	}
	if !canInitialize(successType, valueType, value) {
		a.addErrorAtToken(expressionToken(value), "try handler must produce %s, got %s", typeDisplayName(successType), typeDisplayName(valueType))
		return TryHandlerInvalidFlow
	}
	return TryHandlerProducesValue
}
