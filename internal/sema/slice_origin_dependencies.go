package sema

import "sec/internal/ast"

// knownReturnedReferenceOrigin reads already resolved call/field reference
// relations. It does not infer an expression or resolve a fresh borrow.
// Rules: rules/memory/lifetime_analysis.md — §28; rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–69.
func (a *Analyzer) knownReturnedReferenceOrigin(expr ast.Expression) (localReferenceOrigin, bool) {
	if origin, ok := a.expressionReferenceOrigins[expr]; ok {
		return origin, !origin.Unknown
	}
	switch expr.(type) {
	case *ast.MemberExpression, *ast.IndexExpression:
		return a.containedOriginForAccess(expr)
	}
	return localReferenceOrigin{}, false
}

// returnedReferenceDependencies transports existing root/epoch dependencies
// through call results and contained fields; a range refinement cannot erase a
// local escape restriction or replace an Arena dependency with holder storage.
// Rules: rules/memory/arena.md — §§38,44; rules/memory/lifetime_analysis.md — §28;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–69.
func (a *Analyzer) returnedReferenceDependencies(origin localReferenceOrigin) []ReferenceOriginAlternative {
	var out []ReferenceOriginAlternative
	seen := map[ReferenceOriginAlternative]bool{}
	for _, place := range localOriginPlaces(origin) {
		if symbol, ok := a.symbols[place.Root]; ok && len(symbol.Type.ReferenceOriginAlternatives) > 0 {
			for _, dependency := range symbol.Type.ReferenceOriginAlternatives {
				if !seen[dependency] {
					seen[dependency] = true
					out = append(out, dependency)
				}
			}
			continue
		}
		name, token, _, storage, generation := a.referenceOriginForExpression(&ast.Identifier{Value: place.Root, Token: place.RootToken})
		dependency := ReferenceOriginAlternative{Name: name, Token: token, Local: origin.Local, MatchScoped: origin.MatchScoped, Storage: storage, Generation: generation}
		if !seen[dependency] {
			seen[dependency] = true
			out = append(out, dependency)
		}
	}
	return out
}
