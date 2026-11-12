package sema

import "sec/internal/ast"

// analyzeResultReturnStatement validates the Result arm and its terminal payload
// transfer. The terminal Ok/Err exception permits omission of an explicit move
// marker, but it does not turn an ordinary indexed read into owning extraction.
//
// Rules:
//   - rules/errors/errorhandling.md — §5 "Ok and Err"
//   - rules/errors/errorhandling.md — §5.1 "Direct Option carrier returns"
//   - rules/memory/copy_move.md — §9 "Return boundaries"
//   - rules/memory/copy_move.md — §10.3 "Result construction"
//   - rules/collections/collections.md — §8.5 "Move-only indexed reads"
func (a *Analyzer) analyzeResultReturnStatement(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	if len(returnType.TypeArgs) != 2 {
		return
	}

	// The direct `return try expression` form is a narrow Result forwarding
	// boundary: failure propagates as Err(E), while success is reconstructed as
	// Ok(T). Keeping this check at the return boundary prevents ordinary T
	// expressions from acquiring an implicit T -> Result[T, E] conversion.
	//
	// Rules:
	//   - rules/errors/errorhandling.md — §26 "return try expression"
	//   - rules/memory/copy_move.md — §9 "Return boundaries"
	if tryExpr, ok := stmt.Value.(*ast.TryExpression); ok && len(tryExpr.Handlers) == 0 {
		a.analyzeReturnTryResultForwarding(functionName, returnType, tryExpr)
		return
	}

	switch expr := stmt.Value.(type) {
	case *ast.OkExpression:
		expected := returnType.TypeArgs[0]
		if len(expr.Arguments) > 1 {
			a.addErrorAtToken(expr.Token, "Ok expects 1 argument, got %d", len(expr.Arguments))
			return
		}
		if expr.Value == nil {
			if expected.Kind != VoidType {
				a.addErrorAtToken(expr.Token, "function %s must return Ok(%s), got Ok()", functionName, typeDisplayName(expected))
			}
			return
		}
		// The declared Result success type is also the constructor context. This
		// is required for payload-less generic-union variants such as None, whose
		// concrete Option[T] specialization cannot be recovered from spelling.
		valueType, _ := a.inferExpressionWithExpected(expr.Value, expected)
		if valueType.Kind == InvalidType {
			return
		}
		if a.validateTerminalReturnConstruction(expr.Value) {
			return
		}
		if !a.canInitialize(expected, valueType, expr.Value) {
			a.addTaskTypeMismatchError(expressionToken(expr.Value), expected, valueType, "function %s must return Ok(%s), got Ok(%s)", functionName, typeDisplayName(expected), typeDisplayName(valueType))
			return
		}
		if a.checkDeclaredContractExpression(expected, expr.Value) {
			return
		}
		a.recordFunctionReturnOrigin(expected, expr.Value)
		if a.checkReturningReferenceToLocal(functionName, expected, valueType, expr.Value) {
			return
		}
		if a.checkExpressionEscapesLocalReference(functionName, expr.Value) {
			return
		}
		a.markResourceTransfer(expr.Value)
		a.markMoveSource(expr.Value)
	case *ast.ErrExpression:
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "Err expects 1 argument, got %d", len(expr.Arguments))
			return
		}
		valueType, _ := a.inferExpression(expr.Value)
		if valueType.Kind == InvalidType {
			return
		}
		if a.validateTerminalReturnConstruction(expr.Value) {
			return
		}
		expected := returnType.TypeArgs[1]
		if !a.canInitialize(expected, valueType, expr.Value) {
			a.addTaskTypeMismatchError(expressionToken(expr.Value), expected, valueType, "function %s must return Err(%s), got Err(%s)", functionName, typeDisplayName(expected), typeDisplayName(valueType))
			return
		}
		if a.checkDeclaredContractExpression(expected, expr.Value) {
			return
		}
		a.recordFunctionReturnOrigin(expected, expr.Value)
		a.markMoveSource(expr.Value)
	default:
		valueType, _ := a.inferExpressionWithExpected(stmt.Value, returnType)
		if valueType.Kind == InvalidType {
			return
		}

		// A Result-returning function may directly return an expression that
		// already produces a compatible Result. This is an ordinary return of the
		// complete carrier; it does not implicitly wrap a success value in Ok(...).
		//
		// Rules:
		//   - rules/errors/errorhandling.md — §5 "Ok and Err"
		//   - rules/errors/errorhandling.md — §7 "Result is must-use"
		//   - rules/memory/copy_move.md — §9 "Return boundaries"
		if valueType.Kind == ResultType {
			if !a.canInitialize(returnType, valueType, stmt.Value) {
				a.addTypeMismatchError(expressionToken(stmt.Value), returnType, valueType, stmt.Value, "function %s must return %s, got %s", functionName, typeDisplayName(returnType), typeDisplayName(valueType))
				return
			}

			if a.validateTerminalReturnConstruction(stmt.Value) {
				return
			}

			a.recordFunctionReturnOrigin(returnType, stmt.Value)

			if a.checkReturningReferenceToLocal(functionName, returnType, valueType, stmt.Value) {
				return
			}

			if a.checkExpressionEscapesMatchPayload(functionName, stmt.Value) {
				return
			}

			if a.checkExpressionEscapesLocalReference(functionName, stmt.Value) {
				return
			}

			a.markResourceTransfer(stmt.Value)
			a.markMoveSource(stmt.Value)
			return
		}

		a.addErrorAtToken(expressionToken(stmt.Value), "function %s returning %s must return Ok(...) or Err(...)", functionName, typeDisplayName(returnType))
	}
}
