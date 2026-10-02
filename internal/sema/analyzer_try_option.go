package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

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

// inferHandledOptionTryExpression resolves try over Option[T] with local
// handlers. The only alternate state is None: a `None` handler recovers with a
// value assignable to T, returns, or terminates; Result patterns are rejected
// because absence is not a failure; and when no unguarded None handler exists,
// None propagates only through a compatible enclosing Option return.
//
// Rules:
//   - rules/errors/errorhandling.md — §10.2 "Option[T]", §15 "Local try handlers", §16 "Partial handlers and implicit propagation"
//   - rules/errors/errorhandling.md — §12.2 "Option propagation", §12.3 "No arbitrary cross-channel conversion"
//   - rules/errors/errorhandling.md — §19 "Guards", §21 "Recovery values", §30 "Diagnostics must act as a mentor"
func (a *Analyzer) inferHandledOptionTryExpression(expr *ast.TryExpression, optionType Type) (Type, expressionValue) {
	successType := optionType.TypeArgs[0]
	result := expressionValue{Display: expr.String()}
	plan := ResolvedTryPlan{SuccessType: successType}
	errorsBefore := len(a.errors)
	noneSeen := false
	invalidPattern := false
	var noneToken lexer.Token

	for sourceIndex, handler := range expr.Handlers {
		if !isOptionNonePattern(handler.Pattern) {
			invalidPattern = true
			if _, invalid := handler.Pattern.(*ast.InvalidPattern); !invalid {
				a.addErrorAtToken(handler.Token, "try on %s handles absence with None => ...; %s is not an Option state, and Option absence is never converted to a Result error", typeDisplayName(optionType), handler.Pattern.String())
			}
			continue
		}
		if noneSeen {
			a.addErrorAtTokenWithPreviousID(handler.Token, noneToken, diagnostics.UnreachableTryHandler,
				"unreachable try handler: an earlier None handler already handles absence")
			continue
		}
		guarded := handler.Guard != nil
		if !guarded {
			noneSeen = true
			noneToken = handler.Token
		}
		flow := a.analyzeTryHandlerBody(handler, successType, Type{}, "")
		blockValue := handler.BlockBody != nil && flow == TryHandlerProducesValue && successType.Kind != VoidType
		plan.Handlers = append(plan.Handlers, ResolvedTryHandler{
			PatternKind: TryHandlerOptionNone, Flow: flow, ResultType: successType,
			SourceIndex: sourceIndex, Guarded: guarded, BlockValue: blockValue,
		})
	}

	if noneSeen {
		plan.Exhaustive = true
	} else if !invalidPattern && a.checkOptionResidualPropagation(expr, optionType) {
		plan.ResidualPropagates = true
		plan.EnclosingResultType = a.currentFunctionReturn
	}
	a.resolvedTries[expr] = ResolvedTry{Kind: ResolvedTryHandledOption, SuccessType: successType}
	if len(a.errors) == errorsBefore {
		a.resolvedTryPlans[expr] = plan
	}
	return successType, result
}

// isOptionNonePattern recognizes the None handler pattern, written bare or as
// Option.None.
func isOptionNonePattern(pattern ast.Expression) bool {
	switch pattern := pattern.(type) {
	case *ast.Identifier:
		return pattern.Value == "None"
	case *ast.MemberExpression:
		owner, ok := pattern.Object.(*ast.Identifier)
		return ok && owner.Value == "Option" && pattern.Property != nil && pattern.Property.Value == "None"
	}
	return false
}

// checkOptionResidualPropagation validates that a None left unhandled by
// guarded handlers can propagate through a compatible Option return.
//
// Rules:
//   - rules/errors/errorhandling.md — §12.2 "Option propagation", §16 "Partial handlers and implicit propagation"
func (a *Analyzer) checkOptionResidualPropagation(expr *ast.TryExpression, optionType Type) bool {
	returnType := a.currentFunctionReturn
	switch {
	case a.inDeferBlock:
		a.addErrorAtToken(expr.Token, "try handlers leave None unhandled, and it cannot propagate from inside defer; add an unguarded None => ... handler")
		return false
	case !a.inFunctionBody:
		a.addErrorAtToken(expr.Token, "try handlers leave None unhandled, and it cannot propagate outside a function; add an unguarded None => ... handler")
		return false
	case returnType.Kind != UnionType || returnType.Name != "Option" || len(returnType.TypeArgs) != 1 || !canInitialize(returnType, optionType, expr.Expression):
		a.addErrorAtToken(expr.Token, "try handlers leave None unhandled; it propagates only through an Option return, but this function returns %s; add an unguarded None => ... handler", typeDisplayName(returnType))
		return false
	}
	return true
}
