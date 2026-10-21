package semantic

import (
	"fmt"
	"strings"

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
// from and the field and element steps applied through the reference. An
// explicit `*holder` (MD-063) names the whole referent and adds no step.
func referencePath(expr ast.Expression) (*ast.Identifier, []ast.Expression) {
	var steps []ast.Expression
	for {
		switch step := expr.(type) {
		case *ast.PrefixExpression:
			if holder, ok := step.Right.(*ast.Identifier); ok && step.Operator == "*" {
				return holder, steps
			}
			return nil, nil
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
	place, mutable, _, err := fb.derefHolderPlace(expr, use.Facts.Validity, use.Place, "whole-referent "+string(use.Kind)+" through a reference holder")
	return place, mutable, err
}

// derefHolderPlace dereferences the reference holder at the root of expr once
// (validating a dynamic-epoch reference) and projects the stored fields and
// fixed-array elements of expr below it, returning the Place, its authority
// and the holder's reference facts.
func (fb *functionBuilder) derefHolderPlace(expr ast.Expression, validity sema.ReferenceValidityPolicy, resolved sema.Place, wholeReferent string) (builtValue, bool, *ReferenceMetadata, error) {
	holder, steps := referencePath(expr)
	if holder == nil || len(steps) == 0 && !explicitWholeReferent(expr) {
		return builtValue{}, false, nil, fb.unsupported(wholeReferent, expressionToken(expr))
	}
	reference, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, false, nil, err
	}
	if ReferenceValidity(validity) != metadata.Validity {
		return builtValue{}, false, nil, fb.unsupported("reference use whose validity policy differs from its reference", expressionToken(expr))
	}
	refType, ok := fb.owner.module.Types.Lookup(reference.typ)
	if !ok || refType.Kind != TypeReference {
		return builtValue{}, false, nil, fmt.Errorf("reference holder %s is not a reference value", holder.Value)
	}
	loc := location(holder.Token)
	guard, err := fb.validateReference(reference, metadata, loc)
	if err != nil {
		return builtValue{}, false, nil, err
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: refType.Element, Mutable: refType.Mutable})
	place := fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: metadata, ReferenceGuard: guard, Location: loc}, placeType)
	place, err = fb.projectPlace(steps, resolved.Projections, place, refType.Mutable, placeMetadataOf(resolved))
	return place, refType.Mutable, metadata, err
}

// lowerSubplaceReborrow lowers `ref r.f[i]` / `ref mut r.f[i]` through a
// reference holder: the holder is dereferenced once, the stored fields and
// elements are projected, and ref.reborrow-shared / ref.reborrow-mut derives
// the new borrow from that sub-Place (sec-mlir-dialect_package15.md §§ 46-48,
// 78: a reborrow input may be a derived subplace). Authority never grows: a
// mutable reborrow needs a ref mut holder.
func (fb *functionBuilder) lowerSubplaceReborrow(reference *ast.RefExpression, plan sema.ResolvedReferencePlan) (builtValue, error) {
	place, mutable, metadata, err := fb.derefHolderPlace(reference.Value, plan.Facts.Validity, plan.Place, "whole-referent reborrow through a reference holder")
	if err != nil {
		return builtValue{}, err
	}
	return fb.emitSubplaceReborrow(reference, plan, place, mutable, metadata)
}

// explicitWholeReferent reports a path rooted at an explicit `*holder`, the
// only spelling that reads or replaces a whole referent (MD-063); a bare
// holder never does.
func explicitWholeReferent(expr ast.Expression) bool {
	for {
		switch step := expr.(type) {
		case *ast.MemberExpression:
			expr = step.Object
		case *ast.IndexExpression:
			expr = step.Left
		case *ast.PrefixExpression:
			return step.Operator == "*"
		default:
			return false
		}
	}
}

// lowerReceiverFieldReborrow lowers `ref self.f` / `ref mut f` inside an
// instance method: the receiver is a reference parameter, so the borrow is a
// reborrow of a sub-Place of its dereference.
func (fb *functionBuilder) lowerReceiverFieldReborrow(reference *ast.RefExpression, plan sema.ResolvedReferencePlan, plans []sema.ResolvedStructMemberPlan, steps []ast.Expression) (builtValue, error) {
	if len(steps) == 0 {
		// The whole receiver: reborrow the receiver reference itself.
		place, mutable, err := fb.receiverRootPlace(location(reference.Token))
		if err != nil {
			return builtValue{}, err
		}
		return fb.emitSubplaceReborrow(reference, plan, place, mutable, fb.references[fb.receiver.id])
	}
	place, mutable, err := fb.receiverPlace(plans, steps)
	if err != nil {
		return builtValue{}, err
	}
	return fb.emitSubplaceReborrow(reference, plan, place, mutable, fb.references[fb.receiver.id])
}

// emitSubplaceReborrow derives a new borrow from a sub-Place of the
// dereference of the reference whose facts are source.
func (fb *functionBuilder) emitSubplaceReborrow(reference *ast.RefExpression, plan sema.ResolvedReferencePlan, place builtValue, mutable bool, metadata *ReferenceMetadata) (builtValue, error) {
	if metadata == nil {
		return builtValue{}, fmt.Errorf("reborrowed reference has no recorded facts")
	}
	if reference.Mutable && !mutable {
		return builtValue{}, fmt.Errorf("sub-Place reborrow elevates a shared reference")
	}
	placeType, ok := fb.owner.module.Types.Lookup(place.typ)
	if !ok || placeType.Kind != TypePlace {
		return builtValue{}, fmt.Errorf("reborrowed sub-Place is not a Place")
	}
	derived := referenceMetadataOf(plan.Facts)
	derived.SourceBorrow = metadata.Borrow
	if len(derived.OriginStorages) == 0 {
		derived.OriginStorages = append([]uint32(nil), metadata.OriginStorages...)
	}
	if derived.Borrow == derived.SourceBorrow {
		return builtValue{}, fb.unsupported("sub-Place reborrow without a distinct resolved borrow identity", reference.Token)
	}
	kind, ownership := OpReferenceReborrowShared, OwnershipSharedReference
	if reference.Mutable {
		kind, ownership = OpReferenceReborrowMutable, OwnershipMutableReference
	}
	loc := location(reference.Token)
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: placeType.Element, Mutable: reference.Mutable})
	value := fb.newValue(referenceType, ownership, loc)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{place.id}, Results: []Value{value}, Reference: derived, Location: loc})
	fb.recordReference(value.ID, derived)
	return builtValue{id: value.ID, typ: referenceType}, nil
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
	case sema.ReferenceUseReturn:
		// Returning a holder hands over its own reference SSA value: a
		// shared reference is copyable and a ref mut is moved out by the
		// return, never copied (package15.md §§ 60-61). The origin is
		// checked against the function's return-origin summary.
		holder, named := expr.(*ast.Identifier)
		if !named {
			return builtValue{}, errNotReference
		}
		source, _, err := fb.referenceHolder(holder)
		if err != nil {
			return builtValue{}, err
		}
		if sourceType, ok := fb.owner.module.Types.Lookup(source.typ); !ok || sourceType.Kind != TypeReference || use.Facts.MutableAuthority && !sourceType.Mutable {
			return builtValue{}, fmt.Errorf("returned holder %s does not have the returned authority", holder.Value)
		}
		return source, nil
	}
	return builtValue{}, errNotReference
}

// lowerReferenceWrite lowers an assignment whose target is reached through a
// mutable reference: the destination Place is formed first (one validation),
// then the source is evaluated and written with place.write. A compound
// assignment reads that same Place once, applies the Sema-resolved operator
// with its checks and writes the result only after they pass.
//
// Rules: rules/foundations/operators.md — "Compound assignment";
// rules/memory/references.md — "Explicit whole-value dereference" (MD-063).
func (fb *functionBuilder) lowerReferenceWrite(stmt *ast.AssignmentStatement, use sema.ResolvedReferenceUse) error {
	if stmt.Operator != "=" {
		return fb.lowerReferenceCompoundWrite(stmt, use)
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

func (fb *functionBuilder) lowerReferenceCompoundWrite(stmt *ast.AssignmentStatement, use sema.ResolvedReferenceUse) error {
	resolved, ok := fb.owner.analyzer.ResolvedCompoundAssignmentOf(stmt)
	if !ok || !use.TrivialReplacement {
		return fb.unsupported("compound assignment "+stmt.Operator+" through a reference", stmt.Token)
	}
	if _, ok := stmt.Value.(*ast.TryExpression); ok {
		return fb.unsupported("try arithmetic", expressionToken(stmt.Value))
	}
	place, mutable, err := fb.derefUsePlace(stmt.Target, use)
	if err != nil {
		return err
	}
	if !mutable {
		return fb.unsupported("write through a shared reference", stmt.Token)
	}
	placeType, _ := fb.owner.module.Types.Lookup(place.typ)
	rightType, err := fb.owner.internType(*resolved.RightType)
	if err != nil {
		return err
	}
	resultType, err := fb.owner.internType(resolved.ResultType)
	if err != nil {
		return err
	}
	if resultType != placeType.Element {
		return fmt.Errorf("compound assignment type does not match its referent")
	}
	loc := location(stmt.Token)
	current := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: loc}, placeType.Element)
	value, err := fb.buildExpr(stmt.Value, rightType)
	if err != nil {
		return err
	}
	op := Operation{Operands: []ValueID{current.id, value.id}, Location: loc, Operator: strings.TrimSuffix(stmt.Operator, "=")}
	if !semanticOperatorKind(resolved, &op) {
		return fb.unsupported("compound assignment "+stmt.Operator, stmt.Token)
	}
	updated := builtValue{}
	if resolved.RuntimeCheck || resolved.ShiftCountProven {
		if updated, err = fb.emitCheckedOperator(op, resultType, nil, nil, nil); err != nil {
			return err
		}
	} else {
		updated = fb.result(op, resultType)
	}
	fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, updated.id}, Location: loc})
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

// lowerExplicitReborrow lowers `ref r` / `ref mut r` over a reference holder
// (MD-061) from Sema's reborrow plan: the holder's reference SSA value is
// reborrowed with ref.reborrow-shared or ref.reborrow-mut, deriving a new
// borrow from the holder's borrow and keeping its referent, provenance and
// validity policy. The result is ref T or ref mut T, never a reference to
// the holder; shared-to-mutable elevation was already rejected by Sema.
//
// Rules:
//   - rules/memory/borrowing.md — § 11.4 "Explicit reborrow"
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — reborrow operations
func (fb *functionBuilder) lowerExplicitReborrow(reference *ast.RefExpression, holder *ast.Identifier, plan sema.ResolvedReferencePlan) (builtValue, error) {
	source, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, err
	}
	sourceType, ok := fb.owner.module.Types.Lookup(source.typ)
	if !ok || sourceType.Kind != TypeReference {
		return builtValue{}, fmt.Errorf("reborrowed holder %s is not a reference value", holder.Value)
	}
	if reference.Mutable && !sourceType.Mutable {
		return builtValue{}, fmt.Errorf("explicit reborrow elevates a shared reference")
	}
	derived := referenceMetadataOf(plan.Facts)
	derived.SourceBorrow = metadata.Borrow
	if len(derived.OriginStorages) == 0 {
		// The reborrow keeps the holder's storage dependency (MD-061 § 3(2)).
		derived.OriginStorages = append([]uint32(nil), metadata.OriginStorages...)
	}
	if derived.Validity != metadata.Validity {
		return builtValue{}, fb.unsupported("explicit reborrow whose validity policy differs from its reference", reference.Token)
	}
	if derived.Borrow == derived.SourceBorrow {
		return builtValue{}, fb.unsupported("explicit reborrow without a distinct resolved borrow identity", reference.Token)
	}
	kind, ownership := OpReferenceReborrowShared, OwnershipSharedReference
	if reference.Mutable {
		kind, ownership = OpReferenceReborrowMutable, OwnershipMutableReference
	}
	loc := location(reference.Token)
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: sourceType.Element, Mutable: reference.Mutable})
	value := fb.newValue(referenceType, ownership, loc)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{source.id}, Results: []Value{value}, Reference: derived, Location: loc})
	fb.recordReference(value.ID, derived)
	return builtValue{id: value.ID, typ: referenceType}, nil
}

// dereferenceOfValue reports `*operand` whose operand is a reference value
// rather than a named holder, such as a call returning a reference; Sema
// records no holder use for it.
func dereferenceOfValue(expr ast.Expression, analyzer *sema.Analyzer) (*ast.PrefixExpression, bool) {
	prefix, ok := expr.(*ast.PrefixExpression)
	if !ok || prefix.Operator != "*" {
		return nil, false
	}
	if _, holder := prefix.Right.(*ast.Identifier); holder {
		return nil, false
	}
	if _, used := analyzer.ResolvedReferenceUseOf(expr); used {
		return nil, false
	}
	return prefix, true
}

// lowerValueDereference reads the whole referent of a reference value (MD-063)
// using the facts recorded with that value: one validation, ref.deref-place
// and place.read. Only copy-trivial referents are read here.
func (fb *functionBuilder) lowerValueDereference(prefix *ast.PrefixExpression) (builtValue, error) {
	reference, err := fb.buildExpr(prefix.Right, 0)
	if err != nil {
		return builtValue{}, err
	}
	refType, ok := fb.owner.module.Types.Lookup(reference.typ)
	metadata, known := fb.references[reference.id]
	if !ok || refType.Kind != TypeReference || !known {
		return builtValue{}, fb.unsupported("dereference of a reference value without lowered reference facts", prefix.Token)
	}
	if !fb.storageAllowed(refType.Element) {
		return builtValue{}, fb.unsupported("non-copy-trivial read through a reference", prefix.Token)
	}
	loc := location(prefix.Token)
	guard, err := fb.validateReference(reference, metadata, loc)
	if err != nil {
		return builtValue{}, err
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: refType.Element, Mutable: refType.Mutable})
	place := fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: metadata, ReferenceGuard: guard, Location: loc}, placeType)
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: loc}, refType.Element), nil
}
