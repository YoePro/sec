package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// bindMatchFields projects and binds an arm's shallow named-field bindings on
// the arm's proven variant path (flowcontrol_match.md § 10). A copy is one
// guarded union.unwrap-field of that field only; a borrow is
// place.union-payload, place.field and ref.borrow-* with Sema's reference
// plan, so omitted fields are neither read nor copied. It returns the
// restore function and the borrowed references the arm must end.
func (fb *functionBuilder) bindMatchFields(subjectExpr ast.Expression, subject builtValue, subjectPlace *matchSubjectPlace, armPlan sema.ResolvedMatchArm, matchID MatchID, token lexer.Token) (func(), []builtValue, error) {
	// An `Err(E.Variant { ... })` arm projects from the unwrapped error value
	// that its variant test guards (MD-029).
	variantIndex := UnionVariantIndex(armPlan.UnionVariantIndex)
	errorArm := armPlan.PatternKind == sema.MatchPatternResultErrVariant
	if errorArm {
		variantIndex = fb.errorArm.variant
	}
	definition, ok := fb.owner.unionDefinition(subject.typ)
	if !ok || armPlan.PatternKind != sema.MatchPatternUnionVariant && !(errorArm && fb.errorArm.union) || int(variantIndex) >= len(definition.Variants) {
		return nil, nil, fb.unsupported("named-field match pattern without a resolved union variant", token)
	}
	variant := definition.Variants[variantIndex]
	if variant.Kind != UnionVariantFields {
		return nil, nil, fb.unsupported("named-field match pattern of a variant without named fields", token)
	}
	type saved struct {
		id       sema.BindingID
		previous binding
		existed  bool
	}
	var restores []saved
	restore := func() {
		for index := len(restores) - 1; index >= 0; index-- {
			if restores[index].existed {
				fb.bindings[restores[index].id] = restores[index].previous
			} else {
				delete(fb.bindings, restores[index].id)
			}
		}
	}
	var borrows []builtValue
	for _, field := range armPlan.FieldBindings {
		fieldIndex := -1
		for index, declared := range variant.PayloadFields {
			if declared.Name == field.Field {
				fieldIndex = index
			}
		}
		if fieldIndex < 0 || field.Identifier == nil {
			restore()
			return nil, nil, fmt.Errorf("named-field binding %s has no payload field", field.Field)
		}
		fieldType := variant.PayloadFields[fieldIndex].Type
		var value builtValue
		switch field.Action {
		case sema.MatchBindingCopyTrivial:
			value = fb.result(Operation{
				Kind: OpUnionUnwrapField, Operands: []ValueID{subject.id}, UnionVariant: variantIndex,
				UnionField: field.Field, PayloadActions: []UnionPayloadAction{UnionPayloadCopyTrivial},
				MatchID: matchID, MatchArmIndex: armPlan.SourceIndex, MatchStage: "pattern", MatchPatternKind: string(armPlan.PatternKind),
				Location: location(field.Identifier.Token),
			}, fieldType)
		case sema.MatchBindingBorrowShared, sema.MatchBindingBorrowMutable:
			borrowed, err := fb.borrowMatchField(subjectExpr, subject, subjectPlace, armPlan, field, fieldIndex, fieldType)
			if err != nil {
				restore()
				return nil, nil, err
			}
			value = borrowed
			borrows = append(borrows, borrowed)
		default:
			restore()
			return nil, nil, fb.unsupported("ownership-sensitive named-field match binding "+string(field.Action), field.Identifier.Token)
		}
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(field.Identifier)
		if !ok {
			restore()
			return nil, nil, fmt.Errorf("named-field binding %s has no resolved identity", field.Identifier.Value)
		}
		previous, existed := fb.bindings[fact.ID]
		restores = append(restores, saved{id: fact.ID, previous: previous, existed: existed})
		fb.bindings[fact.ID] = binding{value: value.id, typ: value.typ}
		fb.armPayloadBindings = append(fb.armPayloadBindings, fact.ID)
	}
	return restore, borrows, nil
}

// borrowMatchField borrows one payload field of a subject rooted at a local
// binding or by-value parameter. Sema's plan Place ends in the payload and
// field projections; the union Place is the plan's prefix.
func (fb *functionBuilder) borrowMatchField(subjectExpr ast.Expression, subject builtValue, subjectPlace *matchSubjectPlace, armPlan sema.ResolvedMatchArm, field sema.ResolvedMatchFieldBinding, fieldIndex int, fieldType TypeID) (builtValue, error) {
	token := field.Identifier.Token
	plan, ok := fb.owner.analyzer.ResolvedReferencePlanOf(field.Identifier)
	if !ok {
		return builtValue{}, fb.unsupported("borrowed named field without a resolved reference plan", token)
	}
	if !plan.Facts.Checkable() {
		return builtValue{}, fb.unsupported("uncheckable borrowed named field", token)
	}
	if fb.variantGuard == 0 {
		return builtValue{}, fb.unsupported("borrowed named field without an active-variant test", token)
	}
	if _, _, rooted, _ := fb.receiverFieldPath(subjectExpr); rooted || subjectPlace != nil || plan.Reborrow || plan.Implicit {
		return builtValue{}, fb.unsupported("borrowed named field of a subject reached through the receiver, elements or a reference", token)
	}
	projections := plan.Place.Projections
	if len(projections) < 2 || projections[len(projections)-1].Canonical() != sema.CanonicalProjectionField {
		return builtValue{}, fb.unsupported("borrowed named field without a resolved payload field Place", token)
	}
	unionPlan := plan
	unionPlan.Place.Projections = append([]sema.PlaceProjection(nil), projections[:len(projections)-1]...)
	reference := &ast.RefExpression{Token: token, Value: field.Identifier, Mutable: field.Action == sema.MatchBindingBorrowMutable}
	unionPlace, mutable, err := fb.localSubjectPlace(subjectExpr, reference, unionPlan, true)
	if err != nil {
		return builtValue{}, err
	}
	placeType, ok := fb.owner.module.Types.Lookup(unionPlace.typ)
	if !ok || placeType.Kind != TypePlace || placeType.Element != subject.typ {
		return builtValue{}, fb.unsupported("borrowed named field whose Place is not the match subject", token)
	}
	if reference.Mutable && !mutable {
		return builtValue{}, fb.unsupported("mutable borrow of a read-only union payload field", token)
	}
	variantIndex := UnionVariantIndex(armPlan.UnionVariantIndex)
	payload, ok := variantPayloadType(fb.owner.module, subject.typ, variantIndex)
	if !ok {
		return builtValue{}, fb.unsupported("borrowed named field without a payload Place type", token)
	}
	facts := placeMetadataOf(plan.Place)
	facts.VariantProven = true
	payloadPlace := fb.placeResult(Operation{
		Kind: OpPlaceUnionPayload, Operands: []ValueID{unionPlace.id}, UnionVariant: variantIndex,
		VariantGuard: fb.variantGuard, Place: facts, Location: location(token),
	}, fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: payload, Mutable: mutable}))
	fieldPlace := fb.placeResult(Operation{
		Kind: OpPlaceField, Operands: []ValueID{payloadPlace.id}, StructField: StructFieldID(fieldIndex),
		Place: placeMetadataOf(plan.Place), Location: location(token),
	}, fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: mutable}))
	return fb.emitBorrow(reference, plan, fieldPlace)
}
