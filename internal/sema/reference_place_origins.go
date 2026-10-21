package sema

import "sec/internal/ast"

// referencePlaceOrigin recovers canonical referent Places through reference
// construction, holders and represented projections. A fallible slice retains
// its protected source only when every handler leaves the continuation.
// Rules: rules/memory/reference_model.md — §14(8–11);
// rules/compiler/semantic_ir.md — §126.2,8,22;
// rules/errors/errorhandling.md — §§15–16,21.1;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§21–23,59–62.
func (a *Analyzer) referencePlaceOrigin(expr ast.Expression) (Place, bool) {
	switch expr := expr.(type) {
	case *ast.TryExpression:
		if source, ok := a.continuingSliceTrySource(expr); ok {
			return a.referencePlaceOrigin(source)
		}
		return Place{}, false
	case *ast.RefExpression:
		place, ok := a.resolvePlace(expr.Value)
		if !ok {
			return Place{}, false
		}
		place.ReferenceHolder = ""
		for index := range place.AlternativeOrigins {
			place.AlternativeOrigins[index].ReferenceHolder = ""
		}
		return place, true
	case *ast.SliceExpression:
		place, ok := a.resolvePlace(expr)
		if !ok {
			return Place{}, false
		}
		place.ReferenceHolder = ""
		return place, true
	case *ast.Identifier:
		origin, ok := a.localRefContainers[expr.Value]
		if !ok || origin.Unknown {
			return Place{}, false
		}
		places := localOriginPlaces(origin)
		if len(places) == 0 {
			return Place{}, false
		}
		place := clonePlace(places[0])
		place.ReferenceHolder = expr.Value
		for _, alternative := range places[1:] {
			alternative = clonePlace(alternative)
			alternative.ReferenceHolder = expr.Value
			place.AlternativeOrigins = append(place.AlternativeOrigins, alternative)
		}
		return place, true
	case *ast.CallExpression:
		origin, ok := a.expressionReferenceOrigins[expr]
		if !ok || origin.Unknown {
			return Place{}, false
		}
		return placeFromReferenceOrigin(origin, "")
	case *ast.MemberExpression:
		origin, ok := a.containedOriginForAccess(expr)
		if !ok || origin.Unknown {
			return Place{}, false
		}
		return placeFromReferenceOrigin(origin, mustBorrowRootName(expr))
	case *ast.IndexExpression:
		origin, ok := a.containedOriginForAccess(expr)
		if !ok || origin.Unknown {
			return Place{}, false
		}
		return placeFromReferenceOrigin(origin, mustBorrowRootName(expr))
	case *ast.BitExtractionExpression:
		return a.referencePlaceOrigin(expr.Value)
	case *ast.ConversionExpression:
		return a.referencePlaceOrigin(expr.Value)
	default:
		return Place{}, false
	}
}

// continuingSliceTrySource identifies the sole successful slice source of
// resolved fallible range construction. Recovery values need an origin join;
// missing/invalid plans never establish a single-source proof.
// Rules: rules/errors/errorhandling.md — §§15–16,21.1,35;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§21–23,59–62.
func (a *Analyzer) continuingSliceTrySource(expr *ast.TryExpression) (ast.Expression, bool) {
	resolved, ok := a.resolvedTries[expr]
	if !ok || (resolved.Kind != ResolvedTryFailureSetPropagation && resolved.Kind != ResolvedTryHandledFailureSet) {
		return nil, false
	}
	ref, ok := expr.Expression.(*ast.RefExpression)
	if !ok {
		return nil, false
	}
	if _, sliced := ref.Value.(*ast.SliceExpression); !sliced {
		return nil, false
	}
	if resolved.Kind == ResolvedTryHandledFailureSet {
		plan, ok := a.resolvedTryPlans[expr]
		if !ok || len(plan.Handlers) != len(expr.Handlers) {
			return nil, false
		}
		for _, handler := range plan.Handlers {
			if handler.Flow != TryHandlerReturns && handler.Flow != TryHandlerTerminates {
				return nil, false
			}
		}
	}
	return ref, true
}
