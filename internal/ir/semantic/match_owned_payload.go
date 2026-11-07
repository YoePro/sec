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
	if _, _, stored := fb.storedSubject(subjectExpr); stored {
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
	action, tracked := fb.trackedActions[fact.ID]
	if !tracked {
		return fb.unsupported("owned match payload of an untracked subject (P17-37)", token)
	}
	fb.emitAll(fb.ownedCancels(fact.ID, action, "moved", false, location(token)))
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
	// A named-field variant's whole payload has its own payload Place type,
	// so the guard sees it the same way (P17-37).
	if _, single, found := unionVariant(fb.owner.module, subject.typ, variant); found && single.Kind != UnionVariantSingle && single.Kind != UnionVariantFields {
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
	if _, _, stored := fb.storedSubject(expr); !stored {
		return builtValue{}, nil, false
	}
	place, element, err := fb.storedSubjectPlace(expr)
	if err != nil {
		return builtValue{}, nil, false
	}
	observation := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, PlaceReadMode: PlaceReadObserve, Location: location(expressionToken(expr))}, element)
	return observation, &matchSubjectPlace{place: place, mutable: true}, true
}

// storedSubjectPlace forms the writable Place of a stored subject and its element type.
func (fb *functionBuilder) storedSubjectPlace(expr ast.Expression) (builtValue, TypeID, error) {
	owned, chain, _ := fb.storedSubject(expr)
	place := fb.ownedStoragePlace(owned, expressionToken(expr))
	element := owned.typ
	for _, step := range chain {
		field, _ := fb.owner.analyzer.ResolvedStructMemberOf(step)
		fieldType, err := fb.owner.internType(field.MemberType)
		if err != nil {
			return builtValue{}, 0, err
		}
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: true})
		place = fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(field.FieldID), Place: placeMetadataOf(owned.place), Location: location(step.Token)}, placeType)
		element = fieldType
	}
	return place, element, nil
}

// storedSubject resolves a match subject held in owner storage: the owner itself,
// or a chain of its stored fields, which the owner keeps owning around a moved
// payload (package17 §§ 12, 14, 63).
func (fb *functionBuilder) storedSubject(expr ast.Expression) (ownedStorage, []*ast.MemberExpression, bool) {
	if owned, materialized := fb.materializedSubjects[expr]; materialized {
		return owned, nil, true
	}
	root, chain := memberRoot(expr)
	if root == nil {
		return ownedStorage{}, nil, false
	}
	owned, stored := fb.ownedStorageOf(root)
	if !stored {
		return ownedStorage{}, nil, false
	}
	for _, step := range chain {
		if field, resolved := fb.owner.analyzer.ResolvedStructMemberOf(step); !resolved || field.Kind != sema.MemberStoredField {
			return ownedStorage{}, nil, false
		}
	}
	return owned, chain, true
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
	if arm.PatternKind == sema.MatchPatternResultErrVariant {
		return fb.storedErrorPayload(subjectExpr, arm, token)
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
	owned, _, _ := fb.storedSubject(subjectExpr)
	payload := fb.storedPayloadPlace(owned, place, variant, payloadType, token)
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{payload.id}, Location: location(token)}, payloadType), true, nil
}

// moveStoredPayload moves the active payload out of a mutable owned local. The
// payload was all it held, so its registration ends on this path.
func (fb *functionBuilder) moveStoredPayload(subjectExpr ast.Expression, place builtValue, variant UnionVariantIndex, payloadType TypeID, token lexer.Token) (builtValue, error) {
	identifier, ok := subjectExpr.(*ast.Identifier)
	if !ok {
		// A payload moved out of a stored field leaves the owner registered:
		// its cleanup destroys the remaining fields (package17 §§ 12, 59).
		owned, _, stored := fb.storedSubject(subjectExpr)
		if !stored {
			return builtValue{}, fb.unsupported("owned match payload moved out of a subobject (P17-37)", token)
		}
		payload := fb.storedPayloadPlace(owned, place, variant, payloadType, token)
		return fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{payload.id}, Ownership: &OwnershipFacts{}, Location: location(token)}, payloadType), nil
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
	fb.emitAll(fb.ownedCancels(fact.ID, action, "moved", false, loc))
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
	if _, chain, stored := fb.storedSubject(expr); stored && len(chain) != 0 && fb.owner.maxPackage >= ownershipLoweringPackage {
		// A tried stored field leaves; its owner keeps the remaining fields.
		if typ, ok := fb.owner.analyzer.ResolvedTypeOf(expr); ok && !sema.TriviallyDestructible(typ) {
			place, element, err := fb.storedSubjectPlace(expr)
			if err != nil {
				return builtValue{}, err
			}
			return fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: location(expressionToken(expr))}, element), nil
		}
	}
	if owned, stored := fb.ownedStorageOf(expr); stored {
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(expr.(*ast.Identifier))
		action, tracked := fb.trackedActions[fact.ID]
		if !ok || !tracked {
			return builtValue{}, fb.unsupported("try of an untracked mutable owner", expressionToken(expr))
		}
		token := expressionToken(expr)
		place := fb.ownedStoragePlace(owned, token)
		moved := fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: location(token)}, owned.typ)
		fb.emitAll(fb.ownedCancels(fact.ID, action, "moved", false, location(token)))
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
	fb.emitAll(fb.ownedCancels(fact.ID, action, "moved", false, loc))
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
	if owned, materialized := fb.materializedSubjects[subjectExpr]; materialized {
		// A subject held in storage ends in place, unless the arm took its payload.
		if arm.BindingAction != sema.MatchBindingMove {
			fb.armTemporaries = append(fb.armTemporaries, pendingTemporary{typ: plan.SubjectType, token: token, loops: len(fb.loops), scopes: len(fb.cleanupScopes), stored: &owned})
		}
		return nil
	}
	if fb.owner.maxPackage < ownershipLoweringPackage || !isFreshOwner(subjectExpr) || sema.TriviallyDestructible(plan.SubjectType) || arm.BindingAction == sema.MatchBindingMove {
		return nil
	}
	if arm.BindingAction == sema.MatchBindingBorrowShared || arm.BindingAction == sema.MatchBindingBorrowMutable {
		// Sema publishes a borrowed payload only through the subject's
		// temporary Place, which materializeMatchSubject holds.
		return fb.unsupported("borrowed payload of a fresh match subject outside compiler storage", token)
	}
	if !activePayloadTrivial(plan.SubjectType, arm) {
		// What the arm leaves lives through the whole match (owner decision
		// 2026-11-03); the arm destroys it on every way out.
		fb.holdArmTemporary(subject, plan.SubjectType, token)
		return nil
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

// materializeMatchSubject holds a fresh owned subject in compiler storage when
// an arm tests, takes or borrows a payload that only a Place reaches: a
// concrete error variant's payload, a whole named-field payload, or a `ref` /
// `ref mut` payload binding (owner decision 2026-11-07). The subject is then
// observed like a stored one, and an arm that does not take its payload
// destroys it in place (destruction.md § 8(8)) after the arm's borrows end.
func (fb *functionBuilder) materializeMatchSubject(expr *ast.MatchExpression, plan sema.ResolvedMatchPlan) (builtValue, *matchSubjectPlace, bool, error) {
	borrows := matchBorrowsPayload(plan)
	if fb.owner.maxPackage < ownershipLoweringPackage || !isFreshOwner(expr.Subject) || sema.TriviallyDestructible(plan.SubjectType) && !borrows {
		return builtValue{}, nil, false, nil
	}
	needed := borrows || movesFieldPayload(fb.owner.analyzer, expr)
	for _, arm := range plan.Arms {
		needed = needed || arm.PatternKind == sema.MatchPatternResultErrVariant
	}
	place, known := fb.owner.analyzer.ResolvedMatchSubjectPlaceOf(expr)
	if !needed || !known {
		return builtValue{}, nil, false, nil
	}
	value, err := fb.buildExpr(expr.Subject, 0)
	if err != nil {
		return builtValue{}, nil, false, err
	}
	token := expressionToken(expr.Subject)
	id := fb.nextStorage
	fb.nextStorage++
	fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Type: value.typ, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true, Location: location(token)})
	fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: location(token)})
	owned := ownedStorage{storage: id, place: place, typ: value.typ, scope: fb.currentCleanupScope()}
	root := fb.ownedStoragePlace(owned, token)
	fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{root.id, value.id}, Ownership: &OwnershipFacts{}, Location: location(token)})
	if fb.materializedSubjects == nil {
		fb.materializedSubjects = map[ast.Expression]ownedStorage{}
	}
	fb.materializedSubjects[expr.Subject] = owned
	observation := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{root.id}, PlaceReadMode: PlaceReadObserve, Location: location(token)}, value.typ)
	return observation, &matchSubjectPlace{place: root, mutable: true}, true, nil
}

// storedErrorPayload binds the payload of a concrete error variant through the
// Err payload Place its test observed: a moved payload leaves it, a trivial one
// is read from it.
func (fb *functionBuilder) storedErrorPayload(subjectExpr ast.Expression, arm sema.ResolvedMatchArm, token lexer.Token) (builtValue, bool, error) {
	if !fb.errorArm.union || fb.errorArm.place.id == 0 {
		return builtValue{}, false, fb.unsupported("concrete error variant payload without a union error Place", token)
	}
	errorPlace, _ := fb.owner.module.Types.Lookup(fb.errorArm.place.typ)
	payloadType, ok := variantPayloadType(fb.owner.module, errorPlace.Element, fb.errorArm.variant)
	if !ok {
		return builtValue{}, false, fb.unsupported("concrete error variant payload without a Place type", token)
	}
	owned, _, _ := fb.storedSubject(subjectExpr)
	payload := fb.storedPayloadPlace(owned, fb.errorArm.place, fb.errorArm.variant, payloadType, token)
	if arm.BindingAction == sema.MatchBindingMove {
		return fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{payload.id}, Ownership: &OwnershipFacts{}, Location: location(token)}, payloadType), true, nil
	}
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{payload.id}, Location: location(token)}, payloadType), true, nil
}

// matchBorrowsPayload reports an arm that borrows its payload or a payload field.
func matchBorrowsPayload(plan sema.ResolvedMatchPlan) bool {
	for _, arm := range plan.Arms {
		if arm.BindingAction == sema.MatchBindingBorrowShared || arm.BindingAction == sema.MatchBindingBorrowMutable {
			return true
		}
		for _, field := range arm.FieldBindings {
			if field.Action == sema.MatchBindingBorrowShared || field.Action == sema.MatchBindingBorrowMutable {
				return true
			}
		}
	}
	return false
}

// tryGuardCandidate is a guarded try handler's view of the payload it would
// move: a candidate read through the payload Place of the error value under
// the handler's variant test. The error itself is not touched, so a false
// guard leaves it whole for the next handler. The Place carries the identity
// of the binding the guard names (Sema's ResolvedTryGuardPlaceOf).
func (fb *functionBuilder) tryGuardCandidate(handler *ast.TryHandler, plan sema.ResolvedTryHandler, selected matchErrorArm) (builtValue, error) {
	if !selected.union || selected.test == 0 {
		return builtValue{}, fb.unsupported("guarded owned error payload without a union variant test", handler.Token)
	}
	place, ok := fb.owner.analyzer.ResolvedTryGuardPlaceOf(handler)
	if !ok {
		return builtValue{}, fb.unsupported("guarded owned error payload without a resolved binding Place", handler.Token)
	}
	payloadType, err := fb.owner.internType(plan.BindingType)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(handler.Token)
	rootType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: selected.value.typ})
	base := fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{selected.value.id}, Place: placeMetadataOf(place), Location: loc}, rootType)
	facts := placeMetadataOf(place)
	facts.VariantProven = true
	payloadPlace := fb.placeResult(Operation{
		Kind: OpPlaceUnionPayload, Operands: []ValueID{base.id}, UnionVariant: selected.variant,
		VariantGuard: selected.test, Place: facts, Location: loc,
	}, fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: payloadType}))
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{payloadPlace.id}, PlaceReadMode: PlaceReadCandidate, Location: loc}, payloadType), nil
}
