package sema

import "sec/internal/ast"

// CleanupPreviousValue records the destination availability already resolved
// by Sema after RHS preparation. Conditional/partial states retain their own
// cleanup planning boundary; neither may be treated as a complete old value.
// Rules: rules/memory/copy_move.md — §§12.1–12.3;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,59.
type CleanupPreviousValue string

const (
	CleanupPreviousAvailable   CleanupPreviousValue = "available"
	CleanupPreviousUnavailable CleanupPreviousValue = "unavailable"
	CleanupPreviousConditional CleanupPreviousValue = "conditional"
	CleanupPreviousPartial     CleanupPreviousValue = "partial"
)

// ResolvedOwnedCleanupAssignment preserves success-only replacement/reinit.
// Available requires old-value destruction/cancellation; Unavailable requires
// no old destruction. Conditional/Partial require their path/subobject plans.
// The fresh registration belongs to the original binding's lexical lifetime,
// even when the assignment executes in an inner block or a deferred body.
// Rules: rules/memory/destruction.md — §6.1(2–3), §13;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,38,55,79–80.
type ResolvedOwnedCleanupAssignment struct {
	Previous     CleanupPreviousValue
	Registration ResolvedOwnedCleanupRegistration
	Executor     CallableID
}

// prepareOwnedCleanupAssignment snapshots accepted destination facts after RHS
// checking and before assignment clears moved/partial state. This producer
// currently covers whole local bindings with a declaration registration;
// parameters, projected stores and temporary ownership remain separate owners.
// Rules: rules/memory/copy_move.md — §12.1(2–3), §12.2, §12.3;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,38.
func (a *Analyzer) prepareOwnedCleanupAssignment(stmt *ast.AssignmentStatement, errorsBefore int) *ResolvedOwnedCleanupAssignment {
	if len(a.errors) != errorsBefore || stmt.Operator != "=" {
		return nil
	}
	target, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		return nil
	}
	place, ok := a.rootPlace(target.Value)
	if !ok || TriviallyDestructible(place.Type) {
		return nil
	}
	binding := a.bindingIDs[sourceTokenLocation(place.RootToken)]
	declaration := a.ownedCleanupDeclarations[binding]
	initial, ok := a.ownedCleanupInitializations[declaration]
	if !ok {
		return nil
	}
	previous := CleanupPreviousAvailable
	if _, key, partial, unavailable := a.unavailablePlace(place); unavailable {
		switch {
		case partial:
			previous = CleanupPreviousPartial
		case isConditionalAvailabilityReason(a.moveReasons[key]):
			previous = CleanupPreviousConditional
		default:
			previous = CleanupPreviousUnavailable
		}
	}
	initial.Action = a.cleanupIdentities.owned(binding, stmt.Token)
	initial.Place = cloneSlicePlanPlace(place)
	return &ResolvedOwnedCleanupAssignment{Previous: previous, Registration: initial, Executor: a.currentCallable}
}

// recordOwnedCleanupAssignment publishes only after assignment commit has
// succeeded. Earlier fallible RHS exits preserve the previous responsibility.
// Rules: rules/memory/copy_move.md — §12.1(2–3);
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,38.
func (a *Analyzer) recordOwnedCleanupAssignment(stmt *ast.AssignmentStatement, plan *ResolvedOwnedCleanupAssignment, errorsBefore int) {
	if plan == nil || len(a.errors) != errorsBefore {
		return
	}
	if a.ownedCleanupAssignments == nil {
		a.ownedCleanupAssignments = map[*ast.AssignmentStatement]ResolvedOwnedCleanupAssignment{}
	}
	a.ownedCleanupAssignments[stmt] = *plan
}

// ResolvedOwnedCleanupAssignmentOf returns a detached replacement/reinit fact
// without repeating ownership inference or changing the destination state.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§31–33,79–80.
func (a *Analyzer) ResolvedOwnedCleanupAssignmentOf(stmt *ast.AssignmentStatement) (ResolvedOwnedCleanupAssignment, bool) {
	if a == nil || stmt == nil {
		return ResolvedOwnedCleanupAssignment{}, false
	}
	plan, ok := a.ownedCleanupAssignments[stmt]
	if !ok {
		return ResolvedOwnedCleanupAssignment{}, false
	}
	plan.Registration = cloneOwnedCleanupRegistration(plan.Registration)
	return plan, true
}
