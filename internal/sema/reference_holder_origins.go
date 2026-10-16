package sema

import "sec/internal/ast"

// markLocalRefContainerFromValue retains local and parameter-derived referent
// Places for a reference holder, as well as dependencies nested in aggregates.
// This lets a later alias projection retain the ultimate storage origin when
// the intermediate holder reaches its last use.
// Rules: rules/memory/borrowing.md — §§8.1,12(6),14.4(3);
// rules/collections/collections.md — §§6.5,19.
func (a *Analyzer) markLocalRefContainerFromValue(holder string, value ast.Expression) {
	if holder == "" {
		return
	}
	symbol, ok := a.symbols[holder]
	if !ok || !symbol.Local {
		return
	}
	origin, hasOrigin := a.localReferenceOriginForExpression(value)
	// Parameter-derived references still need canonical referent provenance,
	// even though they are not local-lifetime or match-scoped dependencies.
	if !hasOrigin && symbol.Type.Kind == ReferenceType {
		origin, hasOrigin = a.directReferenceOrigin(value)
	}
	contained := a.containedReferenceOrigins(value)
	if !hasOrigin && len(contained) == 0 {
		return
	}
	origin.Contained = contained
	if !hasOrigin {
		recomputeContainedOriginSummary(&origin)
	}
	a.localRefContainers[holder] = origin
}
