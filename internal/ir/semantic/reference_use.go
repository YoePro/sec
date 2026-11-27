package semantic

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
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

// unwrapOk projects the Ok payload of a Result. The Ok reference of an Arena
// New/Alloc Result states the candidate's dynamic-epoch facts on the unwrap,
// so the verifier can check a locally handled try's join against them (P19
// §§ 35, 151). Uses still read the holder with Sema's facts, exactly like
// the naked try (package15 § 101), so the facts are not recorded as the
// value's own.
func (fb *functionBuilder) unwrapOk(result builtValue, success TypeID, loc Location) builtValue {
	metadata := fb.okReferences[result.id]
	if metadata == nil {
		return fb.result(Operation{Kind: OpResultUnwrapOk, Operands: []ValueID{result.id}, Location: loc}, success)
	}
	value := fb.newValue(success, OwnershipMutableReference, loc)
	fb.emit(Operation{Kind: OpResultUnwrapOk, Operands: []ValueID{result.id}, Results: []Value{value}, Reference: metadata, Location: loc})
	return builtValue{id: value.ID, typ: success}
}

// okUnwrapped reports whether value is an Arena Ok unwrap carrying facts
// (P19 § 35; rules/memory/arena.md § 31).
func (fb *functionBuilder) okUnwrapped(value ValueID) bool {
	for _, block := range fb.fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpResultUnwrapOk && op.Reference != nil && len(op.Results) == 1 && op.Results[0].ID == value {
				return true
			}
		}
	}
	return false
}

// referenceHolder returns the reference SSA value named by holder together
// with the facts it was created with.
func (fb *functionBuilder) referenceHolder(holder *ast.Identifier) (builtValue, *ReferenceMetadata, error) {
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(holder)
	if !ok {
		return builtValue{}, nil, fmt.Errorf("missing resolved binding for %s", holder.Value)
	}
	held, ok := fb.bindings[fact.ID]
	if ok && held.storage != 0 {
		// Holders in mutable storage lower from package 17 (P16 carry-over).
		if fb.owner.maxPackage < ownershipLoweringPackage {
			return builtValue{}, nil, fb.unsupported("reference held in mutable storage", holder.Token)
		}
		return fb.readStoredHolder(holder, held)
	}
	if !ok {
		return builtValue{}, nil, fmt.Errorf("binding %d has no IR value", fact.ID)
	}
	if held.referent != 0 {
		return builtValue{}, nil, fb.unsupported("passing on a reference holder a defer body captured", holder.Token)
	}
	metadata, ok := fb.references[held.value]
	if !ok {
		// A holder bound out of an aggregate (a pattern payload) carries no
		// reference facts of its own: it is read with Sema's facts for this
		// use (package15 § 101).
		if facts, resolved := fb.owner.analyzer.ResolvedHolderReadOf(holder); resolved && facts.Checkable() && held.storage == 0 && held.capture == 0 && fb.owner.maxPackage >= ownershipLoweringPackage {
			root, rooted := fb.owner.analyzer.ResolvedPlaceOf(holder)
			if !rooted {
				return builtValue{}, nil, fb.unsupported("reference holder "+holder.Value+" without a resolved Place", holder.Token)
			}
			root.Projections = nil
			placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: held.typ})
			place := fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{held.value}, Place: placeMetadataOf(root), Location: location(holder.Token)}, placeType)
			value, metadata := fb.holderRead(place, held.typ, facts, location(holder.Token))
			return value, metadata, nil
		}
		return builtValue{}, nil, fb.unsupported("reference holder "+holder.Value+" without lowered reference facts", holder.Token)
	}
	return builtValue{id: held.value, typ: held.typ}, metadata, nil
}

// referenceHolderExpression resolves a named holder or a pack position whose
// expanded parameter holds the reference (rules/memory/references.md § 3.4(10)).
func (fb *functionBuilder) referenceHolderExpression(expr ast.Expression) (builtValue, *ReferenceMetadata, lexer.Token, error) {
	switch holder := expr.(type) {
	case *ast.Identifier:
		value, metadata, err := fb.referenceHolder(holder)
		return value, metadata, holder.Token, err
	case *ast.IndexExpression:
		if value, pack := fb.packIndexValue(holder); pack {
			if metadata, ok := fb.references[value.id]; ok {
				return value, metadata, holder.Token, nil
			}
		}
		if value, metadata, ok, err := fb.readContainedHolder(holder); ok || err != nil {
			return value, metadata, holder.Token, err
		}
	case *ast.MemberExpression:
		if value, metadata, ok, err := fb.readContainedHolder(holder); ok || err != nil {
			return value, metadata, holder.Token, err
		}
	}
	return builtValue{}, nil, expressionToken(expr), fb.unsupported("reference pass of a non-holder expression", expressionToken(expr))
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
	if contained, steps, ok := fb.containedHolderPath(expr); ok {
		reference, metadata, _, err := fb.readContainedHolder(contained)
		if err != nil {
			return builtValue{}, false, nil, err
		}
		return fb.derefReference(expr, reference, metadata, steps, validity, resolved)
	}
	holder, steps := referencePath(expr)
	if holder == nil || len(steps) == 0 && !explicitWholeReferent(expr) {
		return builtValue{}, false, nil, fb.unsupported(wholeReferent, expressionToken(expr))
	}
	if place, mutable, metadata, captured := fb.capturedReferent(holder); captured {
		if ReferenceValidity(validity) != metadata.Validity {
			return builtValue{}, false, nil, fb.unsupported("reference use whose validity policy differs from its reference", expressionToken(expr))
		}
		place, err := fb.projectPlace(steps, resolved.Projections, place, mutable, placeMetadataOf(resolved))
		return place, mutable, metadata, err
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

// derefWholeHolder dereferences a reference holder used as a whole referent,
// such as the subject of `match value` with `value: ref Option[T]`, once
// (validating a dynamic-epoch reference) and returns its referent Place and
// authority.
func (fb *functionBuilder) derefWholeHolder(holder *ast.Identifier) (builtValue, bool, *ReferenceMetadata, error) {
	if place, mutable, metadata, captured := fb.capturedReferent(holder); captured {
		return place, mutable, metadata, nil
	}
	reference, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, false, nil, err
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
	return fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: metadata, ReferenceGuard: guard, Location: loc}, placeType), refType.Mutable, metadata, nil
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
// Rules: rules/memory/reference_model.md — reference-mediated Place access;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§40-45,74-79.
func (fb *functionBuilder) lowerReferenceUse(expr ast.Expression, use sema.ResolvedReferenceUse) (builtValue, error) {
	switch use.Kind {
	case sema.ReferenceUseRead:
		if index, ok := expr.(*ast.IndexExpression); ok && fb.owner.maxPackage >= dynamicArrayLoweringPackage {
			if plan, resolved := fb.owner.analyzer.ResolvedDynamicArrayIndexPlanOf(index); resolved {
				return fb.lowerDynamicArrayIndexRead(index, plan)
			}
		}
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
	if !use.TrivialReplacement && fb.owner.maxPackage < ownershipLoweringPackage {
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
	if !use.TrivialReplacement {
		// A referent reached through a valid `ref mut` is always available;
		// replacement destroys its old value by plan and leaves the new one
		// owned by the referent's owner (destruction.md § 13; package17 § 64).
		concrete := fb.owner.concreteType(use.Place.Type)
		planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(concrete), concrete)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpOwnReplacePlace, Operands: []ValueID{place.id, value.id}, Ownership: &OwnershipFacts{DestructionPlan: planID}, Location: location(stmt.Token)})
		return nil
	}
	fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: location(stmt.Token)})
	return nil
}

func (fb *functionBuilder) lowerReferenceCompoundWrite(stmt *ast.AssignmentStatement, use sema.ResolvedReferenceUse) error {
	resolved, ok := fb.owner.analyzer.ResolvedCompoundAssignmentOf(stmt)
	if !ok || !use.TrivialReplacement {
		return fb.unsupported("compound assignment "+stmt.Operator+" through a reference", stmt.Token)
	}
	// A try source is evaluated after the referent read; its failure edge
	// leaves before the write (operators.md §§ 84(3), 87(3)).
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
	if holder, ok := expr.(*ast.Identifier); ok {
		if referent, mutable, metadata, captured := fb.capturedReferent(holder); captured {
			return fb.passCapturedReferent(holder, referent, mutable, metadata, use)
		}
	}
	source, metadata, token, err := fb.referenceHolderExpression(expr)
	if err != nil {
		return builtValue{}, err
	}
	sourceType, ok := fb.owner.module.Types.Lookup(source.typ)
	if !ok || sourceType.Kind != TypeReference {
		return builtValue{}, fmt.Errorf("passed reference %s is not a reference value", expr.String())
	}
	return fb.passReference(source, sourceType, metadata, referenceMetadataOf(use.Facts), use.Facts.MutableAuthority, token)
}

// passReference copies a shared reference with its borrow or reborrows a
// mutable one under the derived facts, never copying a ref mut.
func (fb *functionBuilder) passReference(source builtValue, sourceType Type, metadata, derived *ReferenceMetadata, mutable bool, token lexer.Token) (builtValue, error) {
	loc := location(token)
	if mutable && !sourceType.Mutable {
		return builtValue{}, fb.unsupported("passing a shared reference to a ref mut parameter", token)
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
	derived.SourceBorrow = metadata.Borrow
	if derived.Validity != metadata.Validity {
		return builtValue{}, fb.unsupported("reference pass whose validity policy differs from its reference", token)
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
func (fb *functionBuilder) lowerExplicitReborrow(reference *ast.RefExpression, holder ast.Expression, plan sema.ResolvedReferencePlan) (builtValue, error) {
	source, metadata, _, err := fb.referenceHolderExpression(holder)
	if err != nil {
		return builtValue{}, err
	}
	sourceType, ok := fb.owner.module.Types.Lookup(source.typ)
	if !ok || sourceType.Kind != TypeReference {
		return builtValue{}, fmt.Errorf("reborrowed holder %s is not a reference value", holder.String())
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
	// A defer body reads the referent of a captured holder through the
	// referent Place established where the defer registered.
	if holder, named := prefix.Right.(*ast.Identifier); named {
		if place, _, _, captured := fb.capturedReferent(holder); captured {
			placeType, ok := fb.owner.module.Types.Lookup(place.typ)
			if !ok || !fb.storageAllowed(placeType.Element) {
				return builtValue{}, fb.unsupported("non-copy-trivial read through a reference", prefix.Token)
			}
			return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: location(prefix.Token)}, placeType.Element), nil
		}
	}
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

// passCapturedReferent passes on a reference holder a defer body captured.
// The body holds the holder's referent Place, never a copy of the holder,
// so the call receives a borrow of that Place derived from the holder's
// borrow under Sema's facts for the pass: shared for a ref parameter,
// mutable only through a ref mut holder (P17-41; borrowing.md § 11).
// Rules: rules/control-flow/defer.md — §§ 9–11; rules/memory/borrowing.md — § 11;
// rules/mlir/packages/sec-mlir-dialect_package15.md — § 60.
func (fb *functionBuilder) passCapturedReferent(holder *ast.Identifier, referent builtValue, mutable bool, metadata *ReferenceMetadata, use sema.ResolvedReferenceUse) (builtValue, error) {
	if use.Facts.MutableAuthority && !mutable {
		return builtValue{}, fb.unsupported("passing a shared reference to a ref mut parameter", holder.Token)
	}
	derived := referenceMetadataOf(use.Facts)
	derived.SourceBorrow = metadata.Borrow
	if derived.Validity != metadata.Validity {
		return builtValue{}, fb.unsupported("reference pass whose validity policy differs from its reference", holder.Token)
	}
	if derived.Borrow == metadata.Borrow {
		return builtValue{}, fb.unsupported("captured reference pass without a distinct resolved borrow identity", holder.Token)
	}
	if len(derived.OriginStorages) == 0 {
		derived.OriginStorages = append([]uint32(nil), metadata.OriginStorages...)
	}
	placeType, _ := fb.owner.module.Types.Lookup(referent.typ)
	// A reborrow of the referent Place the holder was dereferenced into.
	kind, ownership := OpReferenceReborrowShared, OwnershipSharedReference
	if use.Facts.MutableAuthority {
		kind, ownership = OpReferenceReborrowMutable, OwnershipMutableReference
	}
	loc := location(holder.Token)
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: placeType.Element, Mutable: use.Facts.MutableAuthority})
	value := fb.newValue(referenceType, ownership, loc)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{referent.id}, Results: []Value{value}, Reference: derived, Location: loc})
	fb.recordReference(value.ID, derived)
	return builtValue{id: value.ID, typ: referenceType}, nil
}

// readStoredHolder reads a direct reference holder kept in mutable storage at
// one use: the holder may have been rebound (references.md § 3.4(5)), so the
// read carries Sema's facts for this point, whose origins cover every
// reference that can reach it, and the verifier checks them against the
// stores that reach the read. The holder is read out of its Place, never
// copied as a value.
// Rules: rules/memory/references.md — § 3.4(5); rules/memory/borrowing.md — § 8.2;
// rules/memory/lifetime_analysis.md — § 11.
func (fb *functionBuilder) readStoredHolder(holder *ast.Identifier, held binding) (builtValue, *ReferenceMetadata, error) {
	typ, ok := fb.owner.module.Types.Lookup(held.typ)
	if !ok || !isBorrowValueType(fb.owner.module, held.typ) {
		return builtValue{}, nil, fmt.Errorf("stored holder %s is not a reference, slice or view", holder.Value)
	}
	facts, ok := fb.owner.analyzer.ResolvedHolderReadOf(holder)
	if !ok || !facts.Checkable() {
		return builtValue{}, nil, fb.unsupported("reference held in mutable storage without checkable facts at this read", holder.Token)
	}
	root, ok := fb.owner.analyzer.ResolvedPlaceOf(holder)
	if !ok {
		return builtValue{}, nil, fb.unsupported("reference held in mutable storage without a resolved Place", holder.Token)
	}
	root.Projections = nil
	loc := location(holder.Token)
	fb.markStorageAddressTaken(held.storage)
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: held.typ, Mutable: true})
	place := fb.placeResult(Operation{Kind: OpPlaceStorageRoot, Storage: held.storage, Place: placeMetadataOf(root), Location: loc}, placeType)
	metadata := referenceMetadataOf(facts)
	ownership := OwnershipSharedReference
	if typ.Mutable || typ.Kind == TypeMutableSlice {
		ownership = OwnershipMutableReference
	}
	value := fb.newValue(held.typ, ownership, loc)
	fb.emit(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Results: []Value{value}, PlaceReadMode: PlaceReadHolder, Reference: metadata, Location: loc})
	fb.recordReference(value.ID, metadata)
	return builtValue{id: value.ID, typ: held.typ}, metadata, nil
}

// rebindStoredHolder lowers `r = otherRef` (and `r <- other` for a move-only
// mutable holder) to a store into the holder's storage: the holder now holds
// the new reference and nothing is written through the old one (references.md
// § 3.4(5)). The old reference's borrow ends with the holder when no derived
// dependency survives (borrowing.md §§ 8.2, 35.2(3)); later reads carry
// Sema's facts for their point.
func (fb *functionBuilder) rebindStoredHolder(stmt *ast.AssignmentStatement) (bool, error) {
	identifier, ok := stmt.Target.(*ast.Identifier)
	if !ok || stmt.Operator != "=" && stmt.Operator != "<-" || fb.owner.maxPackage < ownershipLoweringPackage {
		return false, nil
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return false, nil
	}
	held, ok := fb.bindings[fact.ID]
	if !ok || held.storage == 0 {
		return false, nil
	}
	typ, ok := fb.owner.module.Types.Lookup(held.typ)
	if !ok || !isBorrowValueType(fb.owner.module, held.typ) {
		return false, nil
	}
	if (typ.Mutable || typ.Kind == TypeMutableSlice) != (stmt.Operator == "<-") {
		return true, fb.unsupported("holder rebind whose operator does not match the holder's authority", stmt.Token)
	}
	value, err := fb.buildExpr(stmt.Value, held.typ)
	if err != nil {
		return true, err
	}
	if value.typ != held.typ {
		return true, fmt.Errorf("rebound reference differs from the holder's type")
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: held.storage, Operands: []ValueID{value.id}, Location: location(stmt.Token)})
	return true, nil
}

// containedHolderPath splits a use whose reference is read out of an
// aggregate (`*h.target`, `h.target.left`, `v.items[0]`) into that contained
// holder expression and the steps applied through the reference.
func (fb *functionBuilder) containedHolderPath(expr ast.Expression) (ast.Expression, []ast.Expression, bool) {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return nil, nil, false
	}
	var steps []ast.Expression
	current := expr
	if prefix, ok := current.(*ast.PrefixExpression); ok && prefix.Operator == "*" {
		current = prefix.Right
	}
	for {
		if _, contained := fb.owner.analyzer.ResolvedContainedReadOf(current); contained {
			return current, steps, true
		}
		switch step := current.(type) {
		case *ast.MemberExpression:
			steps = append([]ast.Expression{step}, steps...)
			current = step.Object
			continue
		case *ast.IndexExpression:
			steps = append([]ast.Expression{step}, steps...)
			current = step.Left
			continue
		}
		return nil, nil, false
	}
}

// derefReference validates a reference once, dereferences it and projects
// the steps below it.
func (fb *functionBuilder) derefReference(expr ast.Expression, reference builtValue, metadata *ReferenceMetadata, steps []ast.Expression, validity sema.ReferenceValidityPolicy, resolved sema.Place) (builtValue, bool, *ReferenceMetadata, error) {
	if ReferenceValidity(validity) != metadata.Validity {
		return builtValue{}, false, nil, fb.unsupported("reference use whose validity policy differs from its reference", expressionToken(expr))
	}
	refType, ok := fb.owner.module.Types.Lookup(reference.typ)
	if !ok || refType.Kind != TypeReference {
		return builtValue{}, false, nil, fmt.Errorf("contained holder of %s is not a reference value", expr.String())
	}
	loc := location(expressionToken(expr))
	guard, err := fb.validateReference(reference, metadata, loc)
	if err != nil {
		return builtValue{}, false, nil, err
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: refType.Element, Mutable: refType.Mutable})
	place := fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: metadata, ReferenceGuard: guard, Location: loc}, placeType)
	place, err = fb.projectPlace(steps, resolved.Projections, place, refType.Mutable, placeMetadataOf(resolved))
	return place, refType.Mutable, metadata, err
}

// readContainedHolder reads a reference, slice or view stored in an
// aggregate out of the aggregate's field or element Place, with the contained
// origin facts Sema resolved for that path (package15 § 101). The aggregate
// is observed in place, never copied, and the holder is never moved out.
func (fb *functionBuilder) readContainedHolder(expr ast.Expression) (builtValue, *ReferenceMetadata, bool, error) {
	facts, ok := fb.owner.analyzer.ResolvedContainedReadOf(expr)
	if !ok || fb.owner.maxPackage < ownershipLoweringPackage {
		return builtValue{}, nil, false, nil
	}
	token := expressionToken(expr)
	if !facts.Checkable() {
		return builtValue{}, nil, true, fb.unsupported("reference read out of an aggregate without checkable facts", token)
	}
	resolved, ok := fb.owner.analyzer.ResolvedPlaceOf(expr)
	if !ok {
		return builtValue{}, nil, true, fb.unsupported("reference read out of an aggregate without a resolved Place", token)
	}
	root, steps := referencePath(expr)
	if root == nil || len(steps) == 0 {
		return builtValue{}, nil, true, fb.unsupported("reference read out of an aggregate path that is not rooted in a binding", token)
	}
	place, mutable, err := fb.bindingRootPlace(root)
	if err != nil {
		return builtValue{}, nil, true, err
	}
	place, err = fb.projectPlace(steps, resolved.Projections, place, mutable, placeMetadataOf(resolved))
	if err != nil {
		return builtValue{}, nil, true, err
	}
	typ, _ := fb.owner.module.Types.Lookup(place.typ)
	value, metadata := fb.holderRead(place, typ.Element, facts, location(token))
	return value, metadata, true, nil
}

// holderRead reads the reference held at place with the facts of this read.
func (fb *functionBuilder) holderRead(place builtValue, typeID TypeID, facts sema.ReferenceFacts, loc Location) (builtValue, *ReferenceMetadata) {
	typ, _ := fb.owner.module.Types.Lookup(typeID)
	ownership := OwnershipSharedReference
	if typ.Mutable || typ.Kind == TypeMutableSlice {
		ownership = OwnershipMutableReference
	}
	metadata := referenceMetadataOf(facts)
	value := fb.newValue(typeID, ownership, loc)
	fb.emit(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Results: []Value{value}, PlaceReadMode: PlaceReadHolder, Reference: metadata, Location: loc})
	fb.recordReference(value.ID, metadata)
	return builtValue{id: value.ID, typ: typeID}, metadata
}

// bindingRootPlace is the root Place of a binding: its owned or mutable
// storage, a defer capture, or the read-only root of an SSA value.
func (fb *functionBuilder) bindingRootPlace(identifier *ast.Identifier) (builtValue, bool, error) {
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return builtValue{}, false, fmt.Errorf("missing resolved binding for %s", identifier.Value)
	}
	bound, ok := fb.bindings[fact.ID]
	if !ok {
		return builtValue{}, false, fmt.Errorf("binding %d has no IR value", fact.ID)
	}
	loc := location(identifier.Token)
	var facts *PlaceMetadata
	if place, resolved := fb.owner.analyzer.ResolvedPlaceOf(identifier); resolved {
		root := place
		root.Projections = nil
		facts = placeMetadataOf(root)
	}
	switch {
	case bound.capture != 0:
		return builtValue{id: bound.capture, typ: fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ, Mutable: bound.mutable})}, bound.mutable, nil
	case fb.isOwnedStorage(fact.ID):
		return fb.ownedStoragePlace(fb.ownedStorages[fact.ID], identifier.Token), true, nil
	case bound.storage != 0:
		fb.markStorageAddressTaken(bound.storage)
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ, Mutable: true})
		return fb.placeResult(Operation{Kind: OpPlaceStorageRoot, Storage: bound.storage, Place: facts, Location: loc}, placeType), true, nil
	default:
		if facts == nil {
			return builtValue{}, false, fb.unsupported("aggregate root without a resolved Place", identifier.Token)
		}
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ})
		return fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{bound.value}, Place: facts, Location: loc}, placeType), false, nil
	}
}
