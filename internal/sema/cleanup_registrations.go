package sema

import "sec/internal/ast"

// CleanupActivation identifies the successful source-semantic point at which
// an ownership obligation starts. It is not an eager runtime registration.
// Rules: rules/memory/destruction.md — §6.1(1);
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§34,38,72.
type CleanupActivation string

const CleanupAfterInitialization CleanupActivation = "after-successful-initialization"

// ResolvedOwnedCleanupRegistration is the obligation established by a valid
// non-trivial local initializer. Activation is on its success continuation:
// an initializer's Err/return/panic paths never register this completed value.
// Destruction retains the canonical plan graph, including named unsupported
// boundaries, rather than treating an unavailable destruction plan as trivial.
// This site fact does not model cancellation, exit edges or dynamic instances.
// Rules: rules/memory/destruction.md — §§3,6.1(1),8;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§19–21,34–38,72,80.
type ResolvedOwnedCleanupRegistration struct {
	Action      CleanupActionID
	Scope       CleanupScopeID
	Invocation  CallableID
	Binding     BindingID
	Place       Place
	Activation  CleanupActivation
	Destruction DestructionPlanSet
}

// recordOwnedCleanupInitialization consumes an accepted initializer only after
// type/contract/ownership validation. Borrowed, static, addressed and trivial
// values do not acquire automatic owned-value destruction through this path.
// Rules: rules/memory/destruction.md — §§3,6.1(1),23,26;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§27,34,38.
func (a *Analyzer) recordOwnedCleanupInitialization(stmt *ast.LetStatement, errorsBefore int) {
	if len(a.errors) != errorsBefore || stmt.Value == nil || stmt.Static || stmt.Address != nil || a.currentCleanupScope == 0 {
		return
	}
	symbol, exists := a.symbols[stmt.Name.Value]
	if !exists || !symbol.Local || symbol.Type.Kind == InvalidType || TriviallyDestructible(symbol.Type) {
		return
	}
	binding := a.bindingIDs[sourceTokenLocation(stmt.Name.Token)]
	if binding == 0 {
		return
	}
	place, ok := a.rootPlace(stmt.Name.Value)
	if !ok {
		return
	}
	registration := ResolvedOwnedCleanupRegistration{
		Action: a.cleanupIdentities.owned(binding, stmt.Name.Token),
		Scope:  a.currentCleanupScope, Invocation: a.currentCallable,
		Binding: binding, Place: cloneSlicePlanPlace(place),
		Activation:  CleanupAfterInitialization,
		Destruction: ResolveDestructionPlans(symbol.Type),
	}
	if a.ownedCleanupInitializations == nil {
		a.ownedCleanupInitializations = map[*ast.LetStatement]ResolvedOwnedCleanupRegistration{}
	}
	a.ownedCleanupInitializations[stmt] = registration
}

// ResolvedOwnedCleanupInitializationOf reads the success-only obligation of a
// validated declaration, detaching its Place/type and complete destruction graph.
// It never infers, allocates identities or changes ownership/borrow state.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§38,79–80.
func (a *Analyzer) ResolvedOwnedCleanupInitializationOf(stmt *ast.LetStatement) (ResolvedOwnedCleanupRegistration, bool) {
	if a == nil || stmt == nil {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	registration, ok := a.ownedCleanupInitializations[stmt]
	if !ok {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	registration.Place = cloneSlicePlanPlace(registration.Place)
	plans := make(map[DestructionPlanID]ResolvedDestructionPlan, len(registration.Destruction.Plans))
	for id, plan := range registration.Destruction.Plans {
		plans[id] = plan.clone()
	}
	registration.Destruction.Plans = plans
	return registration, true
}
