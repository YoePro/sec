package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// analyzeReturnStatement validates an ordinary return and commits its terminal
// ownership transfer. Compatible Result and Option carriers may cross the return
// boundary unchanged; this never implicitly constructs Ok or Some around a
// plain payload.
//
// Rules:
//   - rules/errors/errorhandling.md — §5 "Ok and Err"
//   - rules/errors/errorhandling.md — §5.1 "Direct Option carrier returns"
//   - rules/memory/ownership.md — §17 "Function return boundary"
//   - rules/memory/copy_move.md — §9 "Return boundaries"
//   - rules/declarations/functions.md — §§14 "Return ownership", 32 "Variadic pack cannot escape"
func (a *Analyzer) analyzeReturnStatement(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	// A return is terminal even when reached through a match/try arm. Keeping
	// this as analyzer context lets aggregate and union inference apply the same
	// rule recursively without teaching each constructor about ReturnStatement.
	a.terminalReturnDepth++
	defer func() { a.terminalReturnDepth-- }()
	defer a.recordReferenceReturnUse(stmt.Value, returnType)

	if a.currentFunctionMetadata.Initializer {
		if stmt.Value == nil {
			return
		}
		if _, ok := stmt.Value.(*ast.ErrExpression); ok && a.currentFunctionMetadata.ConstructionError != nil {
			a.analyzeResultReturnStatement("init", returnType, stmt)
			return
		}
		a.addErrorAtToken(expressionToken(stmt.Value), "initializer cannot return a success value; complete the init body to construct self or return Err(error)")
		return
	}
	if a.fallibleSetterBody && functionName != "lambda" {
		a.analyzeFallibleSetterReturn(functionName, returnType, stmt)
		return
	}
	if a.currentFunctionMetadata.CustomFree && functionName != "lambda" && stmt.Value != nil {
		// MD-025.6 (destruction.md § 15.7): free has no result; a bare
		// return is an early exit that still runs field cleanup.
		a.addErrorAtTokenWithMetadata(expressionToken(stmt.Value), "",
			"Use bare `return` to leave free early; the remaining field destruction still runs.",
			"'return' in 'free' cannot return a value")
		return
	}
	if stmt.Value == nil {
		if returnType.Kind != VoidType {
			if functionName == "lambda" {
				a.addErrorAtToken(stmt.Token, "lambda must return %s", typeDisplayName(returnType))
				return
			}
			a.addErrorAtToken(stmt.Token, "function %s must return %s", functionName, typeDisplayName(returnType))
		}
		return
	}

	if a.rejectExplicitReturnStorageMove(stmt) {
		return
	}

	if returnType.Kind == ResultType {
		a.analyzeResultReturnStatement(functionName, returnType, stmt)
		return
	}

	if returnType.Kind == FunctionType {
		if a.checkReturningLambdaCapturingLocalReference(functionName, stmt.Value) {
			return
		}
	}

	if returnType.Kind == FunctionType && containsCapturedLambdaExpression(stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "escaping captured lambda is not supported yet")
		return
	}

	valueType, _ := a.inferExpressionWithExpected(stmt.Value, returnType)
	if valueType.Kind == InvalidType {
		return
	}
	if valueType.Name == "Arena" && a.rejectBorrowedArenaReturn(stmt.Value) {
		return
	}
	if variadicPackValue(valueType) {
		// rules/declarations/functions.md section 32: a pack may not escape
		// the invocation through the function result.
		a.reportEscapeExpressionDiagnostic(stmt.Value, lexer.Token{}, diagnostics.EscapeVariadicPack, "variadic parameter pack cannot escape this call")
		return
	}
	if a.validateTerminalReturnConstruction(stmt.Value) {
		return
	}

	if returnType.Kind == VoidType {
		if functionName == "lambda" {
			a.addErrorAtToken(expressionToken(stmt.Value), "lambda must return void, got %s", typeDisplayName(valueType))
			return
		}
		a.addErrorAtToken(expressionToken(stmt.Value), "function %s must return void, got %s", functionName, typeDisplayName(valueType))
		return
	}

	if a.rejectImplicitBorrow(returnType, valueType, stmt.Value) {
		return
	}
	if !a.canInitialize(returnType, valueType, stmt.Value) && !a.isExplicitUnitConversionReturn(returnType, valueType) {
		if functionName == "lambda" {
			a.addErrorAtToken(expressionToken(stmt.Value), "lambda must return %s, got %s", typeDisplayName(returnType), typeDisplayName(valueType))
			return
		}
		a.addTypeMismatchError(expressionToken(stmt.Value), returnType, valueType, stmt.Value, "function %s must return %s, got %s", functionName, typeDisplayName(returnType), typeDisplayName(valueType))
		return
	}
	if a.checkDeclaredContractExpression(returnType, stmt.Value) {
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
}

// rejectImplicitIndexedReturnExtraction prevents the intrinsically transferring
// return context from turning ordinary collection indexing into an implicit
// move of an element that is not copyable. The collection must instead expose
// an owning extraction operation, or the program must move the containing value
// or return an appropriate borrow.
//
// Rules:
//   - rules/collections/collections.md — "Move-only indexed reads"
//   - rules/memory/copy_move.md — §5.3 "Copy of a copyable field or element"
//   - rules/memory/copy_move.md — §9.1 "Return is intrinsically transferring"
func (a *Analyzer) rejectImplicitIndexedReturnExtraction(expr ast.Expression, elementType Type) bool {
	index, indexed := expr.(*ast.IndexExpression)
	if !indexed || !requiresOwnershipTransfer(elementType) {
		return false
	}
	source := index.String()
	if place, resolved := a.resolvePlace(index); resolved {
		source = place.String()
	}
	a.addErrorAtTokenWithMetadata(
		index.Token,
		diagnostics.ImplicitMoveDisallowed,
		"move the containing collection or return a reference to the element",
		"cannot return %s element %s by ordinary indexing because it is not implicitly copyable",
		typeDisplayName(elementType),
		source,
	)
	return true
}

// validateTerminalReturnConstruction recursively validates only structural
// ownership edges that directly form the returned value. Ordinary calls remain
// governed by their declared parameter modes and are deliberately opaque here.
//
// Rules:
//   - rules/memory/ownership.md — section 17 "Function return boundary"
//   - rules/memory/copy_move.md — sections 9-10
func (a *Analyzer) validateTerminalReturnConstruction(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	switch expr := expr.(type) {
	case *ast.StructLiteral:
		invalid := false
		for _, field := range expr.Fields {
			if field != nil && a.validateTerminalReturnConstruction(field.Value) {
				invalid = true
			}
		}
		return invalid
	case *ast.ArrayLiteral:
		invalid := false
		for _, element := range expr.Elements {
			if a.validateTerminalReturnConstruction(element) {
				invalid = true
			}
		}
		return invalid
	case *ast.OkExpression:
		return a.validateTerminalReturnConstruction(expr.Value)
	case *ast.ErrExpression:
		return a.validateTerminalReturnConstruction(expr.Value)
	case *ast.SpreadExpression:
		return a.validateTerminalReturnConstruction(expr.Value)
	case *ast.BitExtractionExpression:
		return a.validateTerminalReturnConstruction(expr.Value)
	case *ast.ConversionExpression:
		return a.validateTerminalReturnConstruction(expr.Value)
	case *ast.PrefixExpression:
		if expr.Operator == "<-" {
			return false // inferPrefixExpression already validated the explicit move.
		}
	case *ast.CallExpression:
		if _, structural := a.unionConstructorPayload(expr); structural {
			return a.validateTerminalReturnConstruction(expr.Arguments[0])
		}
		return false
	case *ast.MatchExpression:
		// Match arms are checked in their branch-local ownership state. Doing
		// so again after the merge would lose the relevant Place context.
		return false
	}
	typ, ok := a.expressionTypes[expr]
	if !ok || typ.Kind == InvalidType {
		return false
	}
	if a.rejectVariadicPackElementReturn(expr, typ) || a.rejectImplicitIndexedReturnExtraction(expr, typ) {
		return true
	}
	place, reusable := a.resolvePlace(expr)
	if !reusable || !requiresOwnershipTransfer(place.Type) {
		return false
	}
	return !a.validateNamedOwnershipSource(ast.OwnershipMove, expr, expressionToken(expr), false, false, TransferSiteReturn)
}

// rejectVariadicPackElementReturn preserves the invocation-lifetime pack
// boundary before applying the more general move-only collection-read rule.
// A pack element may be observed but cannot be transferred out by a return,
// including a terminal Ok or Err payload return.
//
// Rules:
//   - rules/declarations/functions.md — section 34 "Ownership and lifetime"
//   - rules/collections/collections.md — §8.5 "Move-only indexed reads"
func (a *Analyzer) rejectVariadicPackElementReturn(expr ast.Expression, elementType Type) bool {
	if !a.variadicPackElementExpression(expr) || !requiresOwnershipTransfer(elementType) {
		return false
	}
	a.addErrorAtToken(expressionToken(expr), "cannot move element out of variadic parameter pack")
	return true
}

// isExplicitUnitConversionReturn recognizes the declaration form described by
// rules/types/units.md, "Conversions": a same-named one-argument unit function
// is an explicit conversion boundary and may cross named identity or Kind after
// declaration validation has established compatible dimensions.
func (a *Analyzer) isExplicitUnitConversionReturn(target, source Type) bool {
	fn := a.currentFunctionMetadata
	if a.currentFunctionName != target.Name || fn.ImplTarget != target.Name || target.Unit != target.Name || len(fn.Parameters) != 1 ||
		!hasUnitSemantics(source) || !target.Dimension.Equal(source.Dimension) {
		return false
	}
	return isLinearRatioSemantics(target.UnitSemantics) && isLinearRatioSemantics(source.UnitSemantics)
}

// rejectExplicitReturnStorageMove preserves explicit move restrictions for a
// reusable source while allowing the optional marker on fresh terminal values.
// Copyable self and MMIO observations remain legal ordinary copies, but an
// explicit move must not claim consumption of those special storage roots.
// Rules:
//   - rules/memory/ownership.md — §§9,17,30
//   - rules/memory/copy_move.md — §§18,19.1–19.2
//   - rules/declarations/functions.md — §14
func (a *Analyzer) rejectExplicitReturnStorageMove(stmt *ast.ReturnStatement) bool {
	if stmt.MoveToken.Type != lexer.MOVE_ASSIGN || stmt.Value == nil {
		return false
	}
	place, ok := a.resolvePlace(stmt.Value)
	return ok && (a.rejectOrdinaryMethodWholeSelfConsumption(place, stmt.MoveToken) ||
		a.rejectPhysicalStorageMove(place, stmt.MoveToken))
}
