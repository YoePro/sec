package sema

import "sec/internal/ast"

// rememberStringViewRecovery snapshots a checked recovery value before the
// handler's local symbol scope is restored. Origin queries never re-evaluate it.
// Rules: rules/errors/errorhandling.md — §§9,21–22;
// rules/memory/borrowing.md — §5.5 (MD-126).
func (a *Analyzer) rememberStringViewRecovery(value ast.Expression, typ Type) {
	if typ.Kind != StringViewType {
		return
	}
	if origin, ok := a.directReferenceOrigin(value); ok {
		a.expressionReferenceOrigins[value] = cloneLocalReferenceOrigin(origin)
	}
}

// joinStringViewTryOrigins keeps every possible continuing backing dependency;
// returning/terminating handlers contribute no recovered view.
// Rules: rules/errors/errorhandling.md — §§9,21–22;
// rules/memory/borrowing.md — §5.5 (MD-126).
func (a *Analyzer) joinStringViewTryOrigins(expr *ast.TryExpression, typ Type, plan ResolvedTryPlan) Type {
	if typ.Kind != StringViewType || a.statementTries[expr] {
		return typ
	}
	var origins []localReferenceOrigin
	success, ok := a.directReferenceOrigin(expr.Expression)
	missing := !ok
	if ok {
		origins = append(origins, success)
	}
	for _, handler := range plan.Handlers {
		if handler.Flow != TryHandlerProducesValue {
			continue
		}
		source := expr.Handlers[handler.SourceIndex]
		value := source.Body
		if source.BlockBody != nil {
			value, _ = tryHandlerBlockValue(source.BlockBody)
		}
		origin, known := a.expressionReferenceOrigins[value]
		missing = missing || !known
		if known {
			origins = append(origins, origin)
		}
	}
	joined := mergeLocalReferenceOrigins(origins, missing)
	a.expressionReferenceOrigins[expr] = joined
	typ = a.referenceTypeWithOriginFromExpression(typ, expr)
	// Keep the local representative for escape diagnostics even when the
	// protected expression's first alternative is caller-owned.
	if joined.Local || joined.MatchScoped {
		typ.ReferenceOriginName, typ.ReferenceOriginToken = joined.Name, joined.Token
	}
	return typ
}
