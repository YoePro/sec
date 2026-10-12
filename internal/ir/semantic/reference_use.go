package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Package-15 reference uses (P15-46). Every dereference, read, write and
// equality consumes the reference SSA value of its holder and
// goes through one validity policy taken from that reference's resolved
// facts: a proven reference keeps its proof metadata and emits no runtime
// check; a dynamic-epoch reference is validated once with ref.is-valid on the
// same SSA value, the false edge terminates in fail.reference-generation, and
// the guarded operation exists only on the true edge.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 44-52 (dereference, read/write, reborrow, equality, validity)
//   - rules/memory/reference_model.md — § 45 (stale reference use), § 76(2)
//   - rules/compiler/semantic_ir.md — §§ 10(10)-(11), 45, 96(5)
//   - rules/foundations/operators.md — "Assignment evaluation order"
//
// An explicit `ref r` over a reference holder stays unsupported until MD-061
// decides its meaning; call-site reborrows belong to P15-47.

func (fb *functionBuilder) recordReference(value ValueID, metadata *ReferenceMetadata) {
	if fb.references == nil {
		fb.references = map[ValueID]*ReferenceMetadata{}
	}
	fb.references[value] = metadata
}

// referenceHolder returns the reference SSA value named by holder together
// with the facts it was created with.
func (fb *functionBuilder) referenceHolder(holder *ast.Identifier) (builtValue, *ReferenceMetadata, error) {
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(holder)
	if !ok {
		return builtValue{}, nil, fmt.Errorf("missing resolved binding for %s", holder.Value)
	}
	held, ok := fb.bindings[fact.ID]
	if !ok || held.storage != 0 {
		return builtValue{}, nil, fb.unsupported("reference held in mutable storage", holder.Token)
	}
	metadata, ok := fb.references[held.value]
	if !ok {
		return builtValue{}, nil, fb.unsupported("reference holder "+holder.Value+" without lowered reference facts", holder.Token)
	}
	return builtValue{id: held.value, typ: held.typ}, metadata, nil
}

// validateReference applies the validity policy to one use of reference and
// returns the guard of a dynamic-epoch use (0 for a proven reference).
func (fb *functionBuilder) validateReference(reference builtValue, metadata *ReferenceMetadata, loc Location) (ValueID, error) {
	switch metadata.Validity {
	case ReferenceProven:
		return 0, nil
	case ReferenceDynamicEpoch:
	default:
		return 0, &UnsupportedFeatureError{Feature: "uncheckable safe reference use", Package: fb.owner.maxPackage, Location: loc}
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return 0, err
	}
	guard := fb.result(Operation{Kind: OpReferenceIsValid, Operands: []ValueID{reference.id}, Reference: metadata, Location: loc}, boolType)
	valid, invalid := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{guard.id}, Successors: []BranchTarget{{Block: valid.ID}, {Block: invalid.ID}}, Location: loc})
	fb.current = invalid
	fb.emit(Operation{Kind: OpReferenceGenerationFailure, PanicReasonID: invalidReferenceGenerationID, PanicReason: invalidReferenceGenerationName, Location: loc})
	fb.current = valid
	return guard.id, nil
}

// referencePath splits a use expression into the reference holder it starts
// from and the field and element steps applied through the reference.
func referencePath(expr ast.Expression) (*ast.Identifier, []ast.Expression) {
	var steps []ast.Expression
	for {
		switch step := expr.(type) {
		case *ast.MemberExpression:
			steps = append([]ast.Expression{step}, steps...)
			expr = step.Object
			continue
		case *ast.IndexExpression:
			steps = append([]ast.Expression{step}, steps...)
			expr = step.Left
			continue
		case *ast.Identifier:
			return step, steps
		}
		return nil, nil
	}
}

// derefUsePlace validates the holder's reference once, dereferences it to a
// Place with the reference's authority and projects the steps below it.
func (fb *functionBuilder) derefUsePlace(expr ast.Expression, use sema.ResolvedReferenceUse) (builtValue, bool, error) {
	holder, steps := referencePath(expr)
	if holder == nil || len(steps) == 0 {
		return builtValue{}, false, fb.unsupported("whole-referent "+string(use.Kind)+" through a reference holder", expressionToken(expr))
	}
	reference, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, false, err
	}
	if ReferenceValidity(use.Facts.Validity) != metadata.Validity {
		return builtValue{}, false, fb.unsupported("reference use whose validity policy differs from its reference", expressionToken(expr))
	}
	refType, ok := fb.owner.module.Types.Lookup(reference.typ)
	if !ok || refType.Kind != TypeReference {
		return builtValue{}, false, fmt.Errorf("reference holder %s is not a reference value", holder.Value)
	}
	loc := location(holder.Token)
	guard, err := fb.validateReference(reference, metadata, loc)
	if err != nil {
		return builtValue{}, false, err
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: refType.Element, Mutable: refType.Mutable})
	place := fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: metadata, ReferenceGuard: guard, Location: loc}, placeType)
	place, err = fb.projectPlace(steps, use.Place.Projections, place, refType.Mutable, placeMetadataOf(use.Place))
	return place, refType.Mutable, err
}

// lowerReferenceUse lowers a read or an equality use; any other use kind is
// left to its own lowering or the package-15 boundary.
func (fb *functionBuilder) lowerReferenceUse(expr ast.Expression, use sema.ResolvedReferenceUse) (builtValue, error) {
	switch use.Kind {
	case sema.ReferenceUseRead:
		if !use.CopyTrivialRead {
			return builtValue{}, fb.unsupported("non-copy-trivial read through a reference", expressionToken(expr))
		}
		place, _, err := fb.derefUsePlace(expr, use)
		if err != nil {
			return builtValue{}, err
		}
		placeType, _ := fb.owner.module.Types.Lookup(place.typ)
		return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: locationFromExpression(expr)}, placeType.Element), nil
	case sema.ReferenceUseEquality:
		return fb.lowerReferenceEquality(expr, use)
	case sema.ReferenceUsePass:
		return fb.lowerReferencePass(expr, use)
	}
	return builtValue{}, errNotReference
}

// lowerReferenceWrite lowers an assignment whose target is reached through a
// mutable reference: the destination Place is formed first (one validation),
// then the source is evaluated and written with place.write.
func (fb *functionBuilder) lowerReferenceWrite(stmt *ast.AssignmentStatement, use sema.ResolvedReferenceUse) error {
	if stmt.Operator != "=" {
		return fb.unsupported("compound assignment through a reference", stmt.Token)
	}
	if !use.TrivialReplacement {
		return fb.unsupported("non-trivial replacement through a reference", stmt.Token)
	}
	place, mutable, err := fb.derefUsePlace(stmt.Target, use)
	if err != nil {
		return err
	}
	if !mutable {
		return fb.unsupported("write through a shared reference", stmt.Token)
	}
	placeType, _ := fb.owner.module.Types.Lookup(place.typ)
	value, err := fb.buildExpr(stmt.Value, placeType.Element)
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: location(stmt.Token)})
	return nil
}

// lowerReferenceEquality compares the identities of two direct references
// with ref.compare after validating a dynamic-epoch operand.
func (fb *functionBuilder) lowerReferenceEquality(expr ast.Expression, use sema.ResolvedReferenceUse) (builtValue, error) {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok || (infix.Operator != "==" && infix.Operator != "!=") {
		return builtValue{}, fb.unsupported("reference equality expression", expressionToken(expr))
	}
	left, err := fb.buildExpr(infix.Left, 0)
	if err != nil {
		return builtValue{}, err
	}
	right, err := fb.buildExpr(infix.Right, 0)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(infix.Token)
	var guard ValueID
	for _, operand := range []builtValue{left, right} {
		metadata, ok := fb.references[operand.id]
		if !ok {
			return builtValue{}, fb.unsupported("reference equality operand without lowered reference facts", infix.Token)
		}
		if metadata.Validity != ReferenceDynamicEpoch {
			continue
		}
		if guard != 0 {
			return builtValue{}, fb.unsupported("equality of two dynamic-epoch references", infix.Token)
		}
		if guard, err = fb.validateReference(operand, metadata, loc); err != nil {
			return builtValue{}, err
		}
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	operator := "eq"
	if infix.Operator == "!=" {
		operator = "ne"
	}
	return fb.result(Operation{Kind: OpReferenceCompare, Operands: []ValueID{left.id, right.id}, Operator: operator, ReferenceGuard: guard, Location: loc}, boolType), nil
}

// lowerReferencePass passes an existing reference holder to a ref or ref mut
// parameter (package 15 § 60, borrowing.md § 11): a shared reference to a
// shared parameter is copied with its borrow, a mutable reference to a shared
// parameter is reborrowed shared, and a mutable reference to a ref mut
// parameter is reborrowed mutably for the call, never silently copied.
func (fb *functionBuilder) lowerReferencePass(expr ast.Expression, use sema.ResolvedReferenceUse) (builtValue, error) {
	holder, ok := expr.(*ast.Identifier)
	if !ok {
		return builtValue{}, fb.unsupported("reference pass of a non-holder expression", expressionToken(expr))
	}
	source, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, err
	}
	sourceType, ok := fb.owner.module.Types.Lookup(source.typ)
	if !ok || sourceType.Kind != TypeReference {
		return builtValue{}, fmt.Errorf("passed reference %s is not a reference value", holder.Value)
	}
	loc := location(holder.Token)
	mutable := use.Facts.MutableAuthority
	if mutable && !sourceType.Mutable {
		return builtValue{}, fb.unsupported("passing a shared reference to a ref mut parameter", holder.Token)
	}
	if !mutable && !sourceType.Mutable {
		copied := *metadata
		copied.SourceBorrow = copied.Borrow
		value := fb.newValue(source.typ, OwnershipSharedReference, loc)
		fb.emit(Operation{Kind: OpReferenceCopyShared, Operands: []ValueID{source.id}, Results: []Value{value}, Reference: &copied, Location: loc})
		fb.recordReference(value.ID, &copied)
		return builtValue{id: value.ID, typ: source.typ}, nil
	}
	kind, ownership := OpReferenceReborrowShared, OwnershipSharedReference
	if mutable {
		kind, ownership = OpReferenceReborrowMutable, OwnershipMutableReference
	}
	derived := referenceMetadataOf(use.Facts)
	derived.SourceBorrow = metadata.Borrow
	if derived.Validity != metadata.Validity {
		return builtValue{}, fb.unsupported("reference pass whose validity policy differs from its reference", holder.Token)
	}
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: sourceType.Element, Mutable: mutable})
	value := fb.newValue(referenceType, ownership, loc)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{source.id}, Results: []Value{value}, Reference: derived, Location: loc})
	fb.recordReference(value.ID, derived)
	return builtValue{id: value.ID, typ: referenceType}, nil
}
