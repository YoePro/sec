package sema

import "sec/internal/ast"

// indexSourcePlace resolves the existing storage of an indexed source. A
// returned slice is already a borrowed value, so its canonical referent Place
// supplies authority; it does not require a mutable binding for the call result.
// Owned computed arrays remain non-Places. No expression is inferred or executed
// here, and every alternative origin is retained by sliceSourcePlace.
// Rules: rules/memory/borrowing.md — §§12,14.4–14.5,15;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§43,55,58–62,64–67.
func (a *Analyzer) indexSourcePlace(expr ast.Expression) (Place, bool) {
	if place, ok := a.resolvePlace(expr); ok {
		return place, true
	}
	if _, call := expr.(*ast.CallExpression); !call {
		return Place{}, false
	}
	typ, known := a.expressionTypes[expr]
	if !known || typ.Kind != ReferenceType || typ.Element == nil || typ.Element.Kind != SliceType {
		return Place{}, false
	}
	return a.sliceSourcePlace(expr, typ)
}

// indexAssignmentTargetIsMutable checks the authority of the canonical source
// Place, including the referent of a returned slice, rather than the mutability
// of a temporary reference binding.
// Rules: rules/memory/borrowing.md — §§14.4–14.5;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§55,58–62.
func (a *Analyzer) indexAssignmentTargetIsMutable(expr ast.Expression) bool {
	place, ok := a.indexSourcePlace(expr)
	return ok && place.Mutable
}
