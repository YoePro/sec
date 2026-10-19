package sema

import "sec/internal/ast"

// analyzeIndexAssignmentStatement validates replacement through any resolved
// linear collection element Place, including compiler-known list[T].
//
// Rules:
//   - rules/collections/collections.md — §8.4 "Replacement"
//   - rules/memory/borrowing.md — §§12,14.3–14.4 (projected overlap, not whole-owner read)
func (a *Analyzer) analyzeIndexAssignmentStatement(stmt *ast.AssignmentStatement, index *ast.IndexExpression) {
	targetType, _ := a.inferIndexExpression(index)
	if targetType.Kind == InvalidType {
		return
	}
	a.setResolvedIndexUse(index, ArrayIndexWrite)
	if a.rejectShapedMetadataWrite(index) {
		return
	}
	if packType, _ := a.inferPlaceBase(index.Left); variadicPackValue(packType) {
		// rules/declarations/functions.md section 31: neither the pack nor
		// an element reached through it is mutable source-level storage.
		a.addErrorAtToken(expressionToken(index.Left), "cannot mutate variadic parameter pack")
		return
	}
	if !a.indexAssignmentTargetIsMutable(index.Left) {
		a.addErrorAtToken(expressionToken(index.Left), "cannot assign through immutable index target")
		return
	}
	if place, ok := a.resolvePlace(index); ok && a.checkBorrowedMutationPlace(place, expressionToken(index)) {
		return
	}
	exprType, _ := a.inferExpressionWithExpected(stmt.Value, targetType)
	if exprType.Kind == InvalidType {
		return
	}
	assignmentType := exprType
	if stmt.Operator != "=" {
		var valid bool
		assignmentType, valid = a.inferCompoundAssignmentType(stmt.Operator, targetType, exprType, stmt.Value)
		if !valid {
			return
		}
		a.recordResolvedCompoundAssignment(stmt, targetType, exprType)
	}
	if a.checkStorageDomainStore(stmt.Target, targetType, stmt.Value, exprType) {
		return
	}
	if a.checkAssignmentEscapesLocalReference(stmt.Target, stmt.Value) {
		return
	}
	if !a.validateMoveAssignmentTarget(stmt) {
		return
	}
	if a.checkInitializerType(targetType, assignmentType, stmt.Value) {
		if !a.validateAssignmentOwnership(stmt) {
			return
		}
		a.applyAssignmentOwnership(stmt)
		if stmt.Operator == "=" {
			a.updateContainedOriginsForAssignment(index, stmt.Value)
		}
	}
}
