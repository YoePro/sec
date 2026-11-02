package sema

import "sec/internal/ast"

// ownedCleanupMatchBinding is the registration of a by-value payload binding
// that needs destruction. It lives in a scope of its own around the arm, so
// locals of the arm body are destroyed first and the binding ends with the arm.
// Rules: rules/control-flow/flowcontrol_match.md — §§ 12–13, 18;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 63, 111.
func (a *Analyzer) enterOwnedMatchBinding(arm *ast.MatchArm, info matchPatternInfo) func() {
	if place, ok := a.rootPlace(info.BindingName); ok && arm.Guard != nil && info.BindingName != "" {
		if a.matchGuardPlaces == nil {
			a.matchGuardPlaces = map[*ast.MatchArm]Place{}
		}
		a.matchGuardPlaces[arm] = cloneSlicePlanPlace(place)
	}
	if info.BindingName == "" || TriviallyDestructible(info.BindingType) || resolvedMatchBindingAction(info) != MatchBindingMove {
		return func() {}
	}
	binding := a.bindingIDs[sourceTokenLocation(info.PayloadToken)]
	place, ok := a.rootPlace(info.BindingName)
	if binding == 0 || !ok {
		return func() {}
	}
	if a.matchArmScopes == nil {
		a.matchArmScopes = map[*ast.MatchArm]*ast.BlockStatement{}
	}
	block := a.matchArmScopes[arm]
	if block == nil {
		block = &ast.BlockStatement{Token: arm.Token}
		a.matchArmScopes[arm] = block
	}
	previous := a.currentCleanupScope
	id := a.cleanupIdentities.scope(previous, block.Token)
	if a.cleanupScopes == nil {
		a.cleanupScopes = map[*ast.BlockStatement]CleanupScopeID{}
	}
	a.cleanupScopes[block] = id
	a.currentCleanupScope = id
	a.recordCleanupScope(id, previous, block)
	if a.ownedCleanupMatchBindings == nil {
		a.ownedCleanupMatchBindings = map[*ast.MatchArm]ResolvedOwnedCleanupRegistration{}
	}
	a.ownedCleanupMatchBindings[arm] = ResolvedOwnedCleanupRegistration{
		Action: a.cleanupIdentities.owned(binding, info.PayloadToken),
		Scope:  id, Invocation: a.currentCallable,
		Binding: binding, Place: cloneSlicePlanPlace(place),
		Activation:  CleanupAfterInitialization,
		Destruction: ResolveDestructionPlans(info.BindingType),
	}
	return func() {
		if a.matchArmContinues(arm) {
			a.recordCleanupExit(CleanupExitFallthrough, block.Token, id, []CleanupScopeID{id}, false, nil, "")
		}
		a.currentCleanupScope = previous
	}
}

// matchArmContinues reports an arm body control can fall out of.
func (a *Analyzer) matchArmContinues(arm *ast.MatchArm) bool {
	switch {
	case arm.ReturnBody != nil:
		return false
	case arm.BlockBody != nil:
		return a.blockCanFallThrough(arm.BlockBody)
	case arm.Body != nil:
		typ, ok := a.ResolvedTypeOf(arm.Body)
		return !ok || typ.Kind != NeverType
	}
	return true
}

// ResolvedMatchGuardPlaceOf reads the Place of the binding a guarded arm's guard
// names, whose identity the guard's view of the payload carries.
func (a *Analyzer) ResolvedMatchGuardPlaceOf(arm *ast.MatchArm) (Place, bool) {
	if a == nil {
		return Place{}, false
	}
	place, ok := a.matchGuardPlaces[arm]
	if !ok {
		return Place{}, false
	}
	return cloneSlicePlanPlace(place), true
}

// ResolvedOwnedCleanupOptionBindingOf reads the obligation an `if x is Some(y)`
// binding establishes; the consequence is analyzed as a match arm.
func (a *Analyzer) ResolvedOwnedCleanupOptionBindingOf(stmt *ast.IfStatement) (ResolvedOwnedCleanupRegistration, bool) {
	if a == nil || a.optionBindingArms[stmt] == nil {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	return a.ResolvedOwnedCleanupMatchBindingOf(a.optionBindingArms[stmt])
}

// ResolvedOwnedCleanupMatchBindingOf reads the obligation an arm's moved payload
// binding establishes, detached from analysis state.
func (a *Analyzer) ResolvedOwnedCleanupMatchBindingOf(arm *ast.MatchArm) (ResolvedOwnedCleanupRegistration, bool) {
	if a == nil {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	registration, ok := a.ownedCleanupMatchBindings[arm]
	if !ok {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	return cloneOwnedCleanupRegistration(registration), true
}
