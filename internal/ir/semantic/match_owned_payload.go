package semantic

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// prepareOwnedPayload ends the subject's responsibility for a payload the
// arm moves into its binding. A moved payload is all a union, Option or Result
// held, so a tracked local subject's whole registration ends on this path; a
// fresh subject has none and is taken directly (flowcontrol_match.md § 13).
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 14, 63.
func (fb *functionBuilder) prepareOwnedPayload(subjectExpr ast.Expression, token lexer.Token) error {
	if isFreshOwner(subjectExpr) {
		return nil
	}
	identifier, ok := subjectExpr.(*ast.Identifier)
	if !ok {
		return fb.unsupported("owned match payload moved out of a subobject (P17-37)", token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return fb.unsupported("owned match payload of an unresolved subject", token)
	}
	if _, stored := fb.ownedStorageOf(identifier); stored {
		return nil
	}
	action, tracked := fb.trackedActions[fact.ID]
	if !tracked {
		return fb.unsupported("owned match payload of an untracked subject (P17-37)", token)
	}
	fb.emit(Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: action, Reason: "moved"}, Location: location(token)})
	return nil
}

// guardPayload gives a match guard its view of an arm's payload without taking it:
// a candidate for a payload the arm will move and a trivial read for one it copies
// out of a subject that needs destruction. A false guard leaves the subject whole
// for the next arm. The Place carries the binding's identity, which is what the
// guard names (flowcontrol_match.md § 17).
func (fb *functionBuilder) guardPayload(subject builtValue, subjectPlace *matchSubjectPlace, source *ast.MatchArm, arm sema.ResolvedMatchArm, token lexer.Token) (builtValue, error) {
	variant, ok := payloadVariantIndex(arm)
	if !ok {
		return builtValue{}, fb.unsupported("guarded owned payload binding for "+string(arm.PatternKind), token)
	}
	if _, single, found := unionVariant(fb.owner.module, subject.typ, variant); found && single.Kind != UnionVariantSingle {
		return builtValue{}, fb.unsupported("guarded owned named-field payload binding", token)
	}
	payloadType, ok := variantPayloadType(fb.owner.module, subject.typ, variant)
	if !ok {
		return builtValue{}, fb.unsupported("guarded owned payload without a Place type", token)
	}
	place, ok := fb.owner.analyzer.ResolvedMatchGuardPlaceOf(source)
	if !ok {
		return builtValue{}, fb.unsupported("guarded owned payload without a resolved binding Place", token)
	}
	loc := location(token)
	var base builtValue
	mutable := false
	if subjectPlace != nil {
		base, mutable = subjectPlace.place, subjectPlace.mutable
	} else {
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: subject.typ})
		base = fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{subject.id}, Place: placeMetadataOf(place), Location: loc}, placeType)
	}
	facts := placeMetadataOf(place)
	facts.VariantProven = true
	payloadPlace := fb.placeResult(Operation{
		Kind: OpPlaceUnionPayload, Operands: []ValueID{base.id}, UnionVariant: variant,
		VariantGuard: fb.variantGuard, Place: facts, Location: loc,
	}, fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: payloadType, Mutable: mutable}))
	mode := PlaceReadCandidate
	if arm.BindingAction == sema.MatchBindingCopyTrivial {
		mode = ""
	}
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{payloadPlace.id}, PlaceReadMode: mode, Location: loc}, payloadType), nil
}

// observeOwnedSubject forms the storage Place of a mutable owned subject and an
// observing read for its active-variant tests. Nothing is copied: a payload then
// leaves through the Place (package17 § 63).
func (fb *functionBuilder) observeOwnedSubject(expr ast.Expression) (builtValue, *matchSubjectPlace, bool) {
	owned, stored := fb.ownedStorageOf(expr)
	if !stored {
		return builtValue{}, nil, false
	}
	token := expressionToken(expr)
	place := fb.ownedStoragePlace(owned, token)
	observation := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, PlaceReadMode: PlaceReadObserve, Location: location(token)}, owned.typ)
	return observation, &matchSubjectPlace{place: place, mutable: true}, true
}

// storedMatchPayload binds an arm's payload through the subject's Place: a moved
// one leaves it and a trivially copied one is read from it.
func (fb *functionBuilder) storedMatchPayload(subjectExpr ast.Expression, place, subject builtValue, arm sema.ResolvedMatchArm, token lexer.Token) (builtValue, bool, error) {
	if arm.BindingAction == sema.MatchBindingNone || arm.BindingAction == sema.MatchBindingDiscard {
		return builtValue{}, false, nil
	}
	if arm.BindingAction != sema.MatchBindingMove && arm.BindingAction != sema.MatchBindingCopyTrivial {
		return builtValue{}, false, fb.unsupported("ownership-sensitive match payload binding "+string(arm.BindingAction), token)
	}
	variant, ok := payloadVariantIndex(arm)
	if !ok {
		return builtValue{}, false, fb.unsupported("payload binding for "+string(arm.PatternKind), token)
	}
	payloadType, ok := variantPayloadType(fb.owner.module, subject.typ, variant)
	if !ok {
		return builtValue{}, false, fb.unsupported("owned match payload without a Place type", token)
	}
	if arm.BindingAction == sema.MatchBindingMove {
		value, err := fb.moveStoredPayload(subjectExpr, place, variant, payloadType, token)
		return value, err == nil, err
	}
	owned, _ := fb.ownedStorageOf(subjectExpr)
	payload := fb.storedPayloadPlace(owned, place, variant, payloadType, token)
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{payload.id}, Location: location(token)}, payloadType), true, nil
}

// moveStoredPayload moves the active payload out of a mutable owned local. The
// payload was all it held, so its registration ends on this path.
func (fb *functionBuilder) moveStoredPayload(subjectExpr ast.Expression, place builtValue, variant UnionVariantIndex, payloadType TypeID, token lexer.Token) (builtValue, error) {
	identifier, ok := subjectExpr.(*ast.Identifier)
	if !ok {
		return builtValue{}, fb.unsupported("owned match payload moved out of a subobject (P17-37)", token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return builtValue{}, fb.unsupported("owned match payload of an unresolved subject", token)
	}
	action, tracked := fb.trackedActions[fact.ID]
	if !tracked {
		return builtValue{}, fb.unsupported("owned match payload of an untracked subject (P17-37)", token)
	}
	owned, _ := fb.ownedStorageOf(subjectExpr)
	payload := fb.storedPayloadPlace(owned, place, variant, payloadType, token)
	loc := location(token)
	moved := fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{payload.id}, Ownership: &OwnershipFacts{}, Location: loc}, payloadType)
	fb.emit(Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: action, Reason: "moved"}, Location: loc})
	return moved, nil
}

// storedPayloadPlace projects the active payload under the arm's variant test.
func (fb *functionBuilder) storedPayloadPlace(owned ownedStorage, place builtValue, variant UnionVariantIndex, payloadType TypeID, token lexer.Token) builtValue {
	facts := placeMetadataOf(owned.place)
	facts.VariantProven = true
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: payloadType, Mutable: true})
	return fb.placeResult(Operation{
		Kind: OpPlaceUnionPayload, Operands: []ValueID{place.id}, UnionVariant: variant,
		VariantGuard: fb.variantGuard, Place: facts, Location: location(token),
	}, placeType)
}

// buildTryOperand evaluates a try operand. A tried owner hands its value over on
// both edges, so a tracked local moves out and its registration ends first.
// Rules: rules/memory/destruction.md — §§ 9.1, 16.
func (fb *functionBuilder) buildTryOperand(expr ast.Expression) (builtValue, error) {
	if owned, stored := fb.ownedStorageOf(expr); stored {
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(expr.(*ast.Identifier))
		action, tracked := fb.trackedActions[fact.ID]
		if !ok || !tracked {
			return builtValue{}, fb.unsupported("try of an untracked mutable owner", expressionToken(expr))
		}
		token := expressionToken(expr)
		place := fb.ownedStoragePlace(owned, token)
		moved := fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: location(token)}, owned.typ)
		fb.emit(Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: action, Reason: "moved"}, Location: location(token)})
		return moved, nil
	}
	value, err := fb.buildExpr(expr, 0)
	if err != nil || fb.owner.maxPackage < ownershipLoweringPackage || trivialDestruction(fb.owner.module, value.typ) {
		return value, err
	}
	identifier, ok := expr.(*ast.Identifier)
	if !ok {
		return value, nil
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return value, nil
	}
	action, tracked := fb.trackedActions[fact.ID]
	if !tracked {
		return value, nil
	}
	loc := location(identifier.Token)
	moved := fb.result(Operation{Kind: OpOwnMove, Operands: []ValueID{value.id}, Ownership: &OwnershipFacts{}, Location: loc}, value.typ)
	fb.emit(Operation{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: action, Reason: "moved"}, Location: loc})
	return moved, nil
}

// isFreshOwner reports a subject expression nothing else owns.
func isFreshOwner(expr ast.Expression) bool {
	_, call := expr.(*ast.CallExpression)
	return call
}

// endFreshMatchSubject destroys a fresh subject in an arm that does not take its
// payload. Destroying it here is observable only for an active payload that
// needs destruction, whose full-expression lifetime has no form yet (P17-40).
// Rules: rules/memory/destruction.md — § 8.
func (fb *functionBuilder) endFreshMatchSubject(subjectExpr ast.Expression, subject builtValue, plan sema.ResolvedMatchPlan, arm sema.ResolvedMatchArm, token lexer.Token) error {
	if fb.owner.maxPackage < ownershipLoweringPackage || !isFreshOwner(subjectExpr) || sema.TriviallyDestructible(plan.SubjectType) || arm.BindingAction == sema.MatchBindingMove {
		return nil
	}
	if !activePayloadTrivial(plan.SubjectType, arm) || arm.BindingAction == sema.MatchBindingBorrowShared || arm.BindingAction == sema.MatchBindingBorrowMutable {
		return fb.unsupported("fresh match subject whose active payload outlives the arm (P17-40)", token)
	}
	// An unguarded copy already ended the subject; a guarded one read it through
	// a Place, since a false guard leaves it to the next arm's test.
	if arm.BindingAction == sema.MatchBindingCopyTrivial && !arm.Guarded {
		return nil
	}
	return fb.destroyFreshSubject(subject, plan.SubjectType, token)
}

// destroyFreshSubject ends a fresh container whose active variant holds nothing
// that needs destruction.
func (fb *functionBuilder) destroyFreshSubject(subject builtValue, typ sema.Type, token lexer.Token) error {
	planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(typ), typ)
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpOwnDestroyValue, Operands: []ValueID{subject.id}, Ownership: &OwnershipFacts{Cause: "temporary-end", DestructionPlan: planID}, Location: location(token)})
	return nil
}

// activePayloadTrivial reports an arm whose active variant holds nothing that
// needs destruction. A catch-all can select any remaining variant.
func activePayloadTrivial(subject sema.Type, arm sema.ResolvedMatchArm) bool {
	switch arm.PatternKind {
	case sema.MatchPatternResultOk, sema.MatchPatternResultErr:
		index := 0
		if arm.PatternKind == sema.MatchPatternResultErr {
			index = 1
		}
		return index < len(subject.TypeArgs) && sema.TriviallyDestructible(subject.TypeArgs[index])
	case sema.MatchPatternUnionVariant, sema.MatchPatternOptionSome, sema.MatchPatternOptionNone:
		if int(arm.UnionVariantIndex) >= len(subject.UnionVariants) {
			return false
		}
		variant := subject.UnionVariants[arm.UnionVariantIndex]
		if variant.Payload != nil && !sema.TriviallyDestructible(*variant.Payload) {
			return false
		}
		for _, field := range variant.PayloadFields {
			if !sema.TriviallyDestructible(field.Type) {
				return false
			}
		}
		return true
	}
	return false
}
