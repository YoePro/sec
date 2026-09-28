package sema

import "sec/internal/ast"

// inferNakedOptionTryExpression resolves the Some(T) success type and validates
// that None can leave the current function through the same Option[T] carrier.
// It never maps Option absence to a Result error or changes the payload type.
//
// Rules:
//   - rules/errors/errorhandling.md — §12.2 "Option propagation"
//   - rules/errors/errorhandling.md — §12.3 "No arbitrary cross-channel conversion"
//   - rules/errors/errorhandling.md — §14 "try as a general expression"
func (a *Analyzer) inferNakedOptionTryExpression(expr *ast.TryExpression, optionType Type) (Type, expressionValue) {
	successType := optionType.TypeArgs[0]
	result := expressionValue{Display: expr.String()}

	if a.inDeferBlock {
		a.addErrorAtToken(expr.Token, "bodyless Option try cannot propagate None from inside defer; add a local None handler")
		return successType, result
	}
	if !a.inFunctionBody {
		a.addErrorAtToken(expr.Token, "bodyless Option try cannot propagate None outside a function; add a local None handler")
		return successType, result
	}

	returnType := a.currentFunctionReturn
	if returnType.Kind != UnionType || returnType.Name != "Option" || len(returnType.TypeArgs) != 1 {
		a.addErrorAtToken(
			expr.Token,
			"bodyless Option try cannot propagate None because this function returns %s; return Option[%s] or add a local None handler",
			typeDisplayName(returnType),
			typeDisplayName(successType),
		)
		return successType, result
	}
	if !canInitialize(returnType, optionType, expr.Expression) {
		a.addErrorAtToken(
			expr.Token,
			"bodyless %s try cannot propagate None through incompatible return %s; change the return type or add a local None handler",
			typeDisplayName(optionType),
			typeDisplayName(returnType),
		)
		return successType, result
	}

	a.resolvedTries[expr] = ResolvedTry{
		Kind:                ResolvedTryOptionPropagation,
		SuccessType:         successType,
		EnclosingOptionType: returnType,
	}
	return successType, result
}
