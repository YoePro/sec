package sema

import (
	"sec/internal/ast"
)

// inferRefExpression preserves the shared or mutable access mode on indexed
// element Places after their collection-specific type and bounds resolution.
// Slice borrows are checked again after operand analysis so newly resolved
// returned-reference origins and operand-induced availability changes are safe.
//
// Rules:
//   - rules/collections/collections.md — §8.1 "General rule" and §19 "Borrowing and structural mutation"
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §17
//   - rules/memory/borrowing.md — §§5.3,6–7,12
func (a *Analyzer) inferRefExpression(expr *ast.RefExpression) (Type, expressionValue) {
	if a.rejectPhysicalRegisterBorrow(expr.Value, expr.Token) || a.checkBorrowCreation(expr.Value, expr.Mutable, expr.Token) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if index, ok := expr.Value.(*ast.IndexExpression); ok {
		use := ArrayIndexBorrow
		if expr.Mutable {
			use = ArrayIndexMutBorrow
		}
		a.setResolvedIndexUse(index, use)
	}
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr.Value)
	if place, placed := a.resolvePlace(expr.Value); placed {
		// Slice operands have now been analyzed in order. Validate the final
		// borrow against any operand-induced availability changes, including a
		// call-returned source whose origin was unavailable before inference.
		// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md §17;
		// rules/memory/borrowing.md §§5.3,6–7,12.
		if _, slice := expr.Value.(*ast.SliceExpression); slice && a.checkBorrowCreationPlace(place, expr.Mutable, expr.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}

		a.recordReferencePlan(expr, place, expr.Mutable, false, expr.Token)
		a.recordSliceCreationPlan(expr)
	}
	// MD-061: `ref r`/`ref mut r` over a reference holder is an explicit
	// reborrow of the holder's referent: ref T (or ref mut T), never a
	// reference to the holder (ref ref T).
	if valueType.Kind == ReferenceType && valueType.Element != nil {
		valueType = *valueType.Element
		a.reportExplicitReborrow(expr, originName, valueType)
	}
	return Type{
		Name:                        referenceTypeName(valueType, expr.Mutable),
		Kind:                        ReferenceType,
		Element:                     &valueType,
		ReferenceMutable:            expr.Mutable,
		ReferenceOriginName:         originName,
		ReferenceOriginToken:        originToken,
		ReferenceOriginLocal:        originLocal,
		ReferenceOriginStorage:      originStorage,
		ReferenceOriginGeneration:   generation,
		ReferenceOriginMatchScoped:  a.referenceOriginMatchScopedForExpression(expr.Value),
		ReferenceOriginAlternatives: a.referenceOriginAlternativesForExpression(expr.Value),
	}, expressionValue{Display: expr.String()}
}
