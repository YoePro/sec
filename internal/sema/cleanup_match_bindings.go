package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

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
	if a.matchArmScopes == nil {
		a.matchArmScopes = map[*ast.MatchArm]*ast.BlockStatement{}
	}
	block := a.matchArmScopes[arm]
	if block == nil {
		block = &ast.BlockStatement{Token: arm.Token}
		a.matchArmScopes[arm] = block
	}
	registration, leave, ok := a.enterOwnedBindingScope(block, info.PayloadToken, info.BindingName, info.BindingType, func() bool { return a.matchArmContinues(arm) })
	if !ok {
		return func() {}
	}
	if a.ownedCleanupMatchBindings == nil {
		a.ownedCleanupMatchBindings = map[*ast.MatchArm]ResolvedOwnedCleanupRegistration{}
	}
	a.ownedCleanupMatchBindings[arm] = registration
	return leave
}

// enterOwnedBindingScope opens the cleanup scope of a pattern binding that owns
// a value needing destruction and registers the binding in it. The returned
// function records the scope's fallthrough exit when the body continues and
// closes the scope.
func (a *Analyzer) enterOwnedBindingScope(block *ast.BlockStatement, bindingToken lexer.Token, name string, typ Type, continues func() bool) (ResolvedOwnedCleanupRegistration, func(), bool) {
	binding := a.bindingIDs[sourceTokenLocation(bindingToken)]
	place, ok := a.rootPlace(name)
	if binding == 0 || !ok {
		return ResolvedOwnedCleanupRegistration{}, nil, false
	}
	previous := a.currentCleanupScope
	id := a.cleanupIdentities.scope(previous, block.Token)
	if a.cleanupScopes == nil {
		a.cleanupScopes = map[*ast.BlockStatement]CleanupScopeID{}
	}
	a.cleanupScopes[block] = id
	a.currentCleanupScope = id
	a.recordCleanupScope(id, previous, block)
	registration := ResolvedOwnedCleanupRegistration{
		Action: a.cleanupIdentities.owned(binding, bindingToken),
		Scope:  id, Invocation: a.currentCallable,
		Binding: binding, Place: cloneSlicePlanPlace(place),
		Activation:  CleanupAfterInitialization,
		Destruction: ResolveDestructionPlans(typ),
	}
	return registration, func() {
		if continues() {
			a.recordCleanupExit(CleanupExitFallthrough, block.Token, id, []CleanupScopeID{id}, false, nil, "")
		}
		a.currentCleanupScope = previous
	}, true
}

// enterOwnedHandlerBinding registers a try handler's binding of the whole error
// or of an error variant's payload when it owns a value needing destruction,
// like a match payload binding (errorhandling.md § 20).
func (a *Analyzer) enterOwnedHandlerBinding(handler *ast.TryHandler, name string, typ Type, bindingToken lexer.Token) func() {
	if name == "" || name == "_" || TriviallyDestructible(typ) || CopyClassificationOf(typ) == CopyTrivial {
		return func() {}
	}
	if a.handlerScopes == nil {
		a.handlerScopes = map[*ast.TryHandler]*ast.BlockStatement{}
	}
	block := a.handlerScopes[handler]
	if block == nil {
		block = &ast.BlockStatement{Token: handler.Token}
		a.handlerScopes[handler] = block
	}
	registration, leave, ok := a.enterOwnedBindingScope(block, bindingToken, name, typ, func() bool { return a.tryHandlerContinues(handler) })
	if !ok {
		return func() {}
	}
	if a.ownedCleanupHandlerBindings == nil {
		a.ownedCleanupHandlerBindings = map[*ast.TryHandler]ResolvedOwnedCleanupRegistration{}
	}
	a.ownedCleanupHandlerBindings[handler] = registration
	return leave
}

// tryHandlerContinues reports a handler body control can fall out of.
func (a *Analyzer) tryHandlerContinues(handler *ast.TryHandler) bool {
	switch {
	case handler.ReturnBody != nil:
		return false
	case handler.BlockBody != nil:
		return a.blockCanFallThrough(handler.BlockBody)
	case handler.Body != nil:
		typ, ok := a.ResolvedTypeOf(handler.Body)
		return !ok || typ.Kind != NeverType
	}
	return true
}

// ResolvedOwnedCleanupHandlerBindingOf reads the obligation a try handler's
// owned binding establishes, detached from analysis state.
func (a *Analyzer) ResolvedOwnedCleanupHandlerBindingOf(handler *ast.TryHandler) (ResolvedOwnedCleanupRegistration, bool) {
	if a == nil {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	registration, ok := a.ownedCleanupHandlerBindings[handler]
	if !ok {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	return cloneOwnedCleanupRegistration(registration), true
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

// recordMatchSubjectPlace gives a fresh owned match subject the identity of a
// compiler-materialized temporary Place, which lowering may hold it in while
// its arms test and take its payload through Places (destruction.md § 8(8)).
func (a *Analyzer) recordMatchSubjectPlace(expr *ast.MatchExpression, plan ResolvedMatchPlan) {
	if _, call := expr.Subject.(*ast.CallExpression); !call || TriviallyDestructible(plan.SubjectType) {
		return
	}
	if a.matchSubjectPlaces == nil {
		a.matchSubjectPlaces = map[*ast.MatchExpression]Place{}
	}
	if _, recorded := a.matchSubjectPlaces[expr]; !recorded {
		a.matchSubjectPlaces[expr] = a.temporaryPlace(expr.Subject, plan.SubjectType, true)
	}
}

// ResolvedMatchSubjectPlaceOf reads the temporary Place of a fresh owned match
// subject without allocating anything.
func (a *Analyzer) ResolvedMatchSubjectPlaceOf(expr *ast.MatchExpression) (Place, bool) {
	if a == nil {
		return Place{}, false
	}
	place, ok := a.matchSubjectPlaces[expr]
	if !ok {
		return Place{}, false
	}
	return cloneSlicePlanPlace(place), true
}
