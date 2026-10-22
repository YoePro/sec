package sema

import "sec/internal/ast"

// validateLetOwnership validates a declaration transfer and preserves its
// already defined canonical destination without committing a source move.
// Rules: rules/memory/copy_move.md — §§5.1,10;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§17–18.
func (a *Analyzer) validateLetOwnership(stmt *ast.LetStatement) bool {
	if stmt == nil || a.rejectTemporaryReferenceRetention(stmt.Value) {
		return false
	}
	valid := a.validateNamedOwnershipSource(stmt.Ownership, stmt.Value, stmt.Name.Token, true, stmt.Type == nil, TransferSiteInitialization)
	_, planned := a.resolvedTransferPlans[stmt.Value]
	if valid && planned {
		if place, ok := a.rootPlace(stmt.Name.Value); ok && place.RootToken == stmt.Name.Token {
			a.bindTransferDestinationPlace(stmt.Value, place)
		}
	}
	return valid
}

// validateAssignmentOwnership validates the selected source action and retains
// the canonical target, including independently resolved projected Places.
// Rules: rules/memory/copy_move.md — §§5.2,12;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§17–18.
func (a *Analyzer) validateAssignmentOwnership(stmt *ast.AssignmentStatement) bool {
	if stmt == nil || a.rejectTemporaryReferenceRetention(stmt.Value) {
		return false
	}
	valid := a.validateNamedOwnershipSource(stmt.Ownership, stmt.Value, stmt.Token, false, false, TransferSiteAssignment)
	_, planned := a.resolvedTransferPlans[stmt.Value]
	if valid && planned {
		if place, ok := a.resolvePlace(stmt.Target); ok {
			a.bindTransferDestinationPlace(stmt.Value, place)
		}
	}
	return valid
}
