package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Package-15 reference and Place lowering boundary.
//
// From package 15 on, the builder makes every reference and Place decision
// from the compiler-resolved P15 plans (sema.ResolvedReferencePlanOf,
// ResolvedReferenceUseOf, ResolvedPlaceOf); it never re-resolves a Place,
// infers reference behavior from source syntax, or reruns borrow checking.
// Missing or uncheckable facts are package-tagged unsupported errors, and a
// failed build returns no module, so no partially assembled module or
// placeholder operation escapes.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 33-36 (plans), § 63 (Place/Reference MLIR strategy)
//   - rules/compiler/semantic_ir.md — §§ 1(7), 45, 46, 96(5) and "Unsupported lowerings"

const referenceLoweringPackage = 15

// p15ReferenceBoundary routes an expression that creates, uses or yields a
// direct reference through its resolved plan. handled reports that the
// expression belongs to the reference boundary; the returned error is the
// precise reason it cannot be lowered yet, or the missing-fact rejection.
func (fb *functionBuilder) p15ReferenceBoundary(expr ast.Expression) (bool, error) {
	_, err := fb.p15Reference(expr)
	return err != errNotReference, err
}

// errNotReference reports an expression outside the reference boundary.
var errNotReference = fmt.Errorf("not a reference expression")

// p15Reference lowers an expression on the reference boundary from its
// resolved plan, or explains why it cannot be lowered yet.
func (fb *functionBuilder) p15Reference(expr ast.Expression) (builtValue, error) {
	analyzer := fb.owner.analyzer
	token := expressionToken(expr)
	if value, err := fb.lowerReceiverRead(expr); err != errNotReference {
		return value, err
	}
	if prefix, ok := dereferenceOfValue(expr, analyzer); ok {
		return fb.lowerValueDereference(prefix)
	}
	if call, ok := expr.(*ast.CallExpression); ok {
		if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil && (member.Property.Value == "AsRef" || member.Property.Value == "AsRefMut") {
			if receiver, typed := analyzer.TypeOf(member.Object); typed && receiver.Kind == sema.RawPtrType {
				// MD-070 § 6: a distinct raw-to-safe operation whose
				// serialized form is still MD-084's to spell.
				return builtValue{}, fb.unsupported("raw-to-safe reference conversion RawPtr."+member.Property.Value+" (MD-070 § 6; operation spelling awaits MD-084)", member.Property.Token)
			}
		}
	}
	if plan, ok := analyzer.ResolvedReferencePlanOf(expr); ok {
		if !plan.Facts.Checkable() {
			return builtValue{}, fb.unsupported("uncheckable safe reference (neither proven nor dynamically checkable)", token)
		}
		if plan.Temporary {
			if expr == fb.temporarySource {
				// Producing the owned value itself: no borrow applies yet.
				return builtValue{}, errNotReference
			}
			return fb.lowerTemporaryBorrow(expr, plan)
		}
		if reference, explicit := expr.(*ast.RefExpression); explicit && !plan.Implicit {
			if plans, steps, rooted, ok := fb.receiverFieldPath(reference.Value); rooted && (ok || fb.isReceiverSelf(reference.Value)) {
				return fb.lowerReceiverFieldReborrow(reference, plan, plans, steps)
			}
		}
		if _, explicit := expr.(*ast.RefExpression); !explicit && plan.Implicit {
			// `Read(self)` / `Read(self.inner)` inside a method: the implicit
			// call-site borrow of a receiver-rooted Place is a reborrow of
			// the receiver reference (package15.md §§ 47-48, 59).
			if plans, steps, rooted, ok := fb.receiverFieldPath(expr); rooted && (ok || fb.isReceiverSelf(expr)) {
				implicit := &ast.RefExpression{Token: token, Mutable: plan.Facts.MutableAuthority, Value: expr}
				return fb.lowerReceiverFieldReborrow(implicit, plan, plans, steps)
			}
		}
		if reference, explicit := expr.(*ast.RefExpression); explicit && !plan.Implicit && !plan.Reborrow && onlyFieldProjections(plan.Place) {
			return fb.lowerPlaceBorrow(reference, plan)
		}
		if reference, explicit := expr.(*ast.RefExpression); explicit && !plan.Implicit && plan.Reborrow {
			if holder, named := reference.Value.(*ast.Identifier); named {
				return fb.lowerExplicitReborrow(reference, holder, plan)
			}
			if below, ok := placeBelowDereference(plan.Place); ok && onlyFieldProjections(below) {
				return fb.lowerSubplaceReborrow(reference, plan)
			}
		}
		if _, explicit := expr.(*ast.RefExpression); !explicit && plan.Implicit && !plan.Reborrow && onlyFieldProjections(plan.Place) {
			// package 15 § 59: Sema resolved an implicit borrow of an owned
			// Place passed to a ref or ref mut parameter.
			implicit := &ast.RefExpression{Token: token, Mutable: plan.Facts.MutableAuthority, Value: expr}
			return fb.lowerPlaceBorrow(implicit, plan)
		}
		return builtValue{}, fb.unsupported(fmt.Sprintf("%s reference %s of %s storage", referenceCreationKind(plan), plan.Facts.Kind, plan.Facts.Provenance), token)
	}
	if reference, ok := expr.(*ast.RefExpression); ok {
		return builtValue{}, fb.unsupported("reference creation without a resolved reference plan", reference.Token)
	}
	if use, ok := analyzer.ResolvedReferenceUseOf(expr); ok {
		if !use.Facts.Checkable() {
			return builtValue{}, fb.unsupported("uncheckable safe reference "+string(use.Kind), token)
		}
		if value, err := fb.lowerReferenceUse(expr, use); err != errNotReference {
			return value, err
		}
		return builtValue{}, fb.unsupported("reference "+string(use.Kind)+" lowering", token)
	}
	if call, isCall := expr.(*ast.CallExpression); isCall {
		if _, instantiated := analyzer.ResolvedCallReferenceOf(call); instantiated {
			// A call returning a reference lowers as an ordinary call whose
			// result carries the instantiated caller-side facts.
			return builtValue{}, errNotReference
		}
	}
	if resolved, ok := analyzer.ResolvedTypeOf(expr); ok && resolved.Kind == sema.ReferenceType {
		// Naming an immutable reference holder yields its SSA reference value.
		if identifier, named := expr.(*ast.Identifier); named {
			if fact, bound := analyzer.ResolvedBindingOf(identifier); bound {
				if held, lowered := fb.bindings[fact.ID]; lowered && held.storage == 0 {
					return builtValue{}, errNotReference
				}
			}
		}
		return builtValue{}, fb.unsupported("reference value without a resolved reference plan or use", token)
	}
	return builtValue{}, errNotReference
}

// placeBelowDereference is the part of a reborrowed Place below its single
// dereference of the holder; a Place that dereferences more than once (a
// reference stored in a field) is reported as not lowerable here.
func placeBelowDereference(place sema.Place) (sema.Place, bool) {
	below := place
	below.Projections = nil
	derefs := 0
	for _, projection := range place.Projections {
		if projection.Canonical() == sema.CanonicalProjectionDereference {
			derefs++
			below.Projections = nil
			continue
		}
		below.Projections = append(below.Projections, projection)
	}
	return below, derefs <= 1
}

// onlyFieldProjections reports a Place reached from its root through stored
// struct fields and fixed-array elements only (P15-43, P15-44); union
// payloads and dereferences have their own lowering.
func onlyFieldProjections(place sema.Place) bool {
	for _, projection := range place.Projections {
		switch projection.Canonical() {
		case sema.CanonicalProjectionField, sema.CanonicalProjectionConstant, sema.CanonicalProjectionDynamic:
		default:
			return false
		}
	}
	return true
}

// lowerPlaceBorrow lowers a borrow of a local binding or of a path of stored
// fields and fixed-array elements below it. The root is lowered as for a
// whole binding; every stored field becomes a place.field with its P13 field
// identity from the resolved struct-member plan, and every element a
// place.array-element driven by P14's resolved index plan: the root Place is
// formed first, the index is evaluated exactly once, a proven index forms the
// element directly, and a runtime-checked index emits one
// place.array-index-in-bounds predicate and branches before forming the
// element, reaching the ordinary bounds failure on the false edge. No
// enclosing aggregate is loaded or copied, and authority is preserved along
// the path. A property or any other step is not an addressable Place.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 39-43 (aggregate subplaces, element borrows), §§ 68-70
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — §§ 32-36 (bounds plans and failure)
//   - rules/compiler/semantic_ir.md — §§ 10(8), 10(11)
func (fb *functionBuilder) lowerPlaceBorrow(reference *ast.RefExpression, plan sema.ResolvedReferencePlan) (builtValue, error) {
	place, err := fb.lowerBorrowPlace(reference, plan)
	if err != nil {
		return builtValue{}, err
	}
	return fb.emitBorrow(reference, plan, place)
}

// lowerBorrowPlace evaluates the canonical source path without loading its
// aggregate. Direct-reference and slice borrows share this Place producer.
// Rules: rules/compiler/semantic_ir.md — §96(5), §126.7;
// rules/mlir/packages/sec-mlir-dialect_package15.md — §§39–43;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §42.
func (fb *functionBuilder) lowerBorrowPlace(reference *ast.RefExpression, plan sema.ResolvedReferencePlan) (builtValue, error) {
	var steps []ast.Expression
	current := reference.Value
	for {
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
		break
	}
	if len(steps) != len(plan.Place.Projections) {
		return builtValue{}, fb.unsupported("reference Place path that differs from its resolved Place", reference.Token)
	}
	for index, step := range steps {
		projection := plan.Place.Projections[index]
		switch step := step.(type) {
		case *ast.MemberExpression:
			resolved, ok := fb.owner.analyzer.ResolvedStructMemberOf(step)
			if !ok || resolved.Kind != sema.MemberStoredField {
				return builtValue{}, fb.unsupported("borrow of a property or non-stored member (properties are not addressable field Places)", step.Token)
			}
			if projection.Canonical() != sema.CanonicalProjectionField || resolved.FieldName != projection.Name {
				return builtValue{}, fb.unsupported("field borrow whose member plan differs from its resolved Place", step.Token)
			}
		case *ast.IndexExpression:
			indexPlan, ok := fb.owner.analyzer.ResolvedArrayIndexPlanOf(step)
			if !ok || projection.Kind != sema.PlaceIndex {
				return builtValue{}, fb.unsupported("element borrow without a resolved fixed-array index plan", step.Token)
			}
			if _, fixed := sema.FixedArrayLength(indexPlan.ArrayType); !fixed {
				return builtValue{}, fb.unsupported("element borrow of a non-fixed-array sequence", step.Token)
			}
		}
	}
	root, err := fb.lowerRootPlace(reference, plan, current)
	if err != nil {
		return builtValue{}, err
	}
	mutablePlace := false
	if rootType, ok := fb.owner.module.Types.Lookup(root.typ); ok {
		mutablePlace = rootType.Mutable
	}
	place, err := fb.projectPlace(steps, plan.Place.Projections, root, mutablePlace, placeMetadataOf(plan.Place))
	if err != nil {
		return builtValue{}, err
	}
	if reference.Mutable && !mutablePlace {
		return builtValue{}, fb.unsupported("mutable borrow of a read-only Place", reference.Token)
	}
	return place, nil
}

// projectPlace applies already validated field and fixed-array element steps
// to a Place, preserving its authority; projections holds the resolved Place
// projections the steps correspond to, aligned at their end.
func (fb *functionBuilder) projectPlace(steps []ast.Expression, projections []sema.PlaceProjection, place builtValue, mutable bool, facts *PlaceMetadata) (builtValue, error) {
	offset := len(projections) - len(steps)
	if offset < 0 {
		return builtValue{}, fmt.Errorf("Place path longer than its resolved projections")
	}
	for index, step := range steps {
		projection := projections[offset+index]
		switch step := step.(type) {
		case *ast.MemberExpression:
			resolved, ok := fb.owner.analyzer.ResolvedStructMemberOf(step)
			if !ok || resolved.Kind != sema.MemberStoredField || projection.Canonical() != sema.CanonicalProjectionField || resolved.FieldName != projection.Name {
				return builtValue{}, fb.unsupported("Place step that is not the resolved stored field", step.Token)
			}
			fieldType, err := fb.owner.internType(resolved.MemberType)
			if err != nil {
				return builtValue{}, err
			}
			placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: mutable})
			place = fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(resolved.FieldID), Place: facts, Location: location(step.Token)}, placeType)
		case *ast.IndexExpression:
			indexPlan, ok := fb.owner.analyzer.ResolvedArrayIndexPlanOf(step)
			if !ok || projection.Kind != sema.PlaceIndex {
				return builtValue{}, fb.unsupported("element Place without a resolved fixed-array index plan", step.Token)
			}
			if _, fixed := sema.FixedArrayLength(indexPlan.ArrayType); !fixed {
				return builtValue{}, fb.unsupported("element Place of a non-fixed-array sequence", step.Token)
			}
			var err error
			place, err = fb.lowerElementPlace(step, projection, place, mutable, facts)
			if err != nil {
				return builtValue{}, err
			}
		default:
			return builtValue{}, fb.unsupported("Place step", expressionToken(step))
		}
	}
	return place, nil
}

// lowerElementPlace forms one fixed-array element Place from the evaluated
// array Place and P14's resolved index plan (P15-44).
func (fb *functionBuilder) lowerElementPlace(expr *ast.IndexExpression, projection sema.PlaceProjection, array builtValue, mutable bool, facts *PlaceMetadata) (builtValue, error) {
	indexPlan, _ := fb.owner.analyzer.ResolvedArrayIndexPlanOf(expr)
	elementType, err := fb.owner.internType(indexPlan.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	indexType, err := fb.owner.internType(indexPlan.IndexType)
	if err != nil {
		return builtValue{}, err
	}
	index, err := fb.buildExpr(expr.Index, indexType)
	if err != nil {
		return builtValue{}, err
	}
	if index.typ != indexType {
		return builtValue{}, fmt.Errorf("fixed-array element index operand type mismatch")
	}
	elementFacts := *facts
	elementFacts.DynamicIndexID = uint32(projection.DynamicIndexID)
	op := Operation{Kind: OpPlaceArrayElement, Operands: []ValueID{array.id, index.id}, ArrayIndexSigned: indexPlan.IndexSigned, Place: &elementFacts, Location: location(expr.Token)}
	if !projection.DynamicIndex && projection.ConstantIndex != nil {
		op.Integer = new(big.Int).Set(projection.ConstantIndex)
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: elementType, Mutable: mutable})
	if indexPlan.CheckKind == sema.ArrayIndexProvenSafe {
		proof, proofOK := semanticArrayIndexProof(indexPlan.ProofKind)
		if !proofOK || indexPlan.FailureMode != sema.ArrayIndexFailureNone {
			return builtValue{}, fmt.Errorf("invalid proven-safe fixed-array element plan")
		}
		op.ArrayCheckKind, op.ArrayProofKind = ArrayIndexProvenSafe, proof
		return fb.placeResult(op, placeType), nil
	}
	if indexPlan.CheckKind != sema.ArrayIndexRuntimeCheck || indexPlan.FailureMode != sema.ArrayIndexFailureOrdinary {
		return builtValue{}, fb.unsupported("non-ordinary fixed-array element borrow failure mode", expr.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	predicate := fb.result(Operation{Kind: OpPlaceArrayIndexInBounds, Operands: []ValueID{array.id, index.id}, ArrayIndexSigned: indexPlan.IndexSigned, Location: location(expr.Token)}, boolType)
	success := fb.newBlock()
	failure := fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{predicate.id}, Successors: []BranchTarget{{Block: success.ID}, {Block: failure.ID}}, Location: location(expr.Token)})
	fb.current = failure
	fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: "fixed-array-index", Location: location(expr.Token)})
	fb.current = success
	op.ArrayCheckKind, op.ArrayProofKind, op.ArrayGuard = ArrayIndexRuntimeCheck, ArrayIndexProofGuarded, predicate.id
	return fb.placeResult(op, placeType), nil
}

// lowerRootPlace lowers the root Place of a local binding: an immutable
// binding is a read-only Place over its SSA value, a mutable binding a Place
// over its high-level P5 storage, which is marked address-taken so trivial-
// core lowering cannot erase it. Static and addressed roots have no Semantic
// IR storage representation yet and stay unsupported.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 64-67 (Place roots, address-taken storage, P5 guard)
//   - rules/compiler/semantic_ir.md — §§ 10(7)-(9), 46(6)
func (fb *functionBuilder) lowerRootPlace(reference *ast.RefExpression, plan sema.ResolvedReferencePlan, rootExpr ast.Expression) (builtValue, error) {
	switch plan.Facts.Provenance {
	case sema.ReferenceProvenanceLocal, sema.ReferenceProvenanceParameter:
		// A by-value parameter is an immutable SSA root like a local; reference
		// parameters never reach this point (P15-47).
	case sema.ReferenceProvenanceStatic:
		return fb.lowerStaticRootPlace(reference, plan, rootExpr)
	default:
		return builtValue{}, fb.unsupported(fmt.Sprintf("%s Place root", plan.Facts.Provenance), reference.Token)
	}
	if plan.Facts.AddressStability == sema.AddressStabilityFixed || plan.Facts.AddressSpace != sema.AddressSpaceOrdinary {
		return builtValue{}, fb.unsupported("addressed or non-ordinary Place root", reference.Token)
	}
	identifier, ok := rootExpr.(*ast.Identifier)
	if !ok {
		return builtValue{}, fb.unsupported("reference root expression", reference.Token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return builtValue{}, fmt.Errorf("missing resolved binding for %s", identifier.Value)
	}
	held, ok := fb.bindings[fact.ID]
	if !ok {
		return builtValue{}, fmt.Errorf("binding %d has no IR value", fact.ID)
	}
	loc := location(identifier.Token)
	placeFacts := placeMetadataOf(plan.Place)
	if held.storage != 0 {
		index := -1
		for position, storage := range fb.fn.Storages {
			if storage.ID == held.storage {
				index = position
			}
		}
		if index < 0 {
			return builtValue{}, fmt.Errorf("binding %d storage $%d is not declared", fact.ID, held.storage)
		}
		fb.fn.Storages[index].AddressTaken = true
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: held.typ, Mutable: fb.fn.Storages[index].Mutable})
		return fb.placeResult(Operation{Kind: OpPlaceStorageRoot, Storage: held.storage, Place: placeFacts, Location: loc}, placeType), nil
	}
	if reference.Mutable {
		return builtValue{}, fb.unsupported("mutable borrow of an immutable binding", reference.Token)
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: held.typ})
	return fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{held.value}, Place: placeFacts, Location: loc}, placeType), nil
}

// emitBorrow creates the direct reference over an already lowered Place with
// the plan's reference facts unchanged.
func (fb *functionBuilder) emitBorrow(reference *ast.RefExpression, plan sema.ResolvedReferencePlan, place builtValue) (builtValue, error) {
	placeType, ok := fb.owner.module.Types.Lookup(place.typ)
	if !ok || placeType.Kind != TypePlace {
		return builtValue{}, fmt.Errorf("borrow operand is not a Place")
	}
	kind, ownership := OpReferenceBorrowShared, OwnershipSharedReference
	if reference.Mutable {
		kind, ownership = OpReferenceBorrowMutable, OwnershipMutableReference
	}
	loc := location(reference.Token)
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: placeType.Element, Mutable: reference.Mutable})
	value := fb.newValue(referenceType, ownership, loc)
	metadata := referenceMetadataOf(plan.Facts)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{place.id}, Results: []Value{value}, Reference: metadata, Location: loc})
	fb.recordReference(value.ID, metadata)
	return builtValue{id: value.ID, typ: referenceType}, nil
}

// placeMetadataOf copies the resolved Place root facts into Semantic IR.
func placeMetadataOf(place sema.Place) *PlaceMetadata {
	space := AddressSpaceOrdinary
	if place.AddressSpace == sema.AddressSpaceMMIO {
		space = AddressSpaceMMIO
	}
	return &PlaceMetadata{Root: uint32(place.RootID), RootKind: PlaceRootKind(place.RootKind), Storage: uint32(place.StorageIdentity), AddressSpace: space}
}

// referenceMetadataOf copies resolved reference facts into Semantic IR
// without re-deriving any of them.
func referenceMetadataOf(facts sema.ReferenceFacts) *ReferenceMetadata {
	space := AddressSpaceOrdinary
	if facts.AddressSpace == sema.AddressSpaceMMIO {
		space = AddressSpaceMMIO
	}
	metadata := &ReferenceMetadata{
		Borrow: uint32(facts.Borrow), Lifetime: uint32(facts.Lifetime),
		Validity: ReferenceValidity(facts.Validity), ProofProvenance: facts.ProofProvenance,
		InvalidationDomain: uint32(facts.InvalidationDomain), EpochDependency: uint32(facts.EpochDependency),
		AddressSpace: space, AddressStability: string(facts.AddressStability), Pinned: facts.Pinned, Provenance: string(facts.Provenance),
	}
	for _, storage := range facts.StorageIdentities {
		metadata.OriginStorages = append(metadata.OriginStorages, uint32(storage))
	}
	return metadata
}

func referenceCreationKind(plan sema.ResolvedReferencePlan) string {
	switch {
	case plan.Implicit:
		return "implicit call-site borrow of"
	case plan.Reborrow:
		return "reborrow of"
	default:
		return "borrow of"
	}
}

// p15ReferenceParameter returns the callee-side reference facts of a ref or
// ref mut parameter from Sema (package 15 § 59), or nil for any other
// parameter. The pre-P15 representation (the referent value with reference
// ownership) is never used at package 15.
func (fb *functionBuilder) p15ReferenceParameter(parameter sema.FunctionParameter) (*ReferenceMetadata, error) {
	if !parameter.Ref && !parameter.MutableRef && parameter.Type.Kind != sema.ReferenceType {
		return nil, nil
	}
	facts, ok := fb.owner.analyzer.ReferenceParameterFactsOf(parameter)
	if !ok {
		return nil, fb.unsupported("reference parameter "+parameter.Name+" without resolved reference facts", parameter.Token)
	}
	if !facts.Checkable() {
		return nil, fb.unsupported("uncheckable reference parameter "+parameter.Name, parameter.Token)
	}
	return referenceMetadataOf(facts), nil
}

// referenceArgumentAction reports the argument action of a reference-typed
// parameter.
func (fb *functionBuilder) referenceArgumentAction(parameter TypeID) (ArgumentAction, bool) {
	typ, ok := fb.owner.module.Types.Lookup(parameter)
	if !ok || typ.Kind != TypeReference {
		return "", false
	}
	if typ.Mutable {
		return ArgumentMutableReference, true
	}
	return ArgumentSharedReference, true
}

// internReferenceType maps a resolved `ref T` / `ref mut T` source type to the
// P15 Semantic IR reference type; earlier packages reject it.
func (b *builder) internReferenceType(t sema.Type) (TypeID, error) {
	if b.maxPackage < referenceLoweringPackage || t.Element == nil {
		return 0, &UnsupportedFeatureError{Feature: "reference type " + t.Name, Package: b.maxPackage}
	}
	element, err := b.internType(*t.Element)
	if err != nil {
		return 0, err
	}
	return b.module.Types.Intern(Type{Kind: TypeReference, Element: element, Mutable: t.ReferenceMutable}), nil
}

// placeResult emits a Place-producing operation; a Place is compiler-internal
// and never an immediate or owned language value.
func (fb *functionBuilder) placeResult(op Operation, typ TypeID) builtValue {
	value := fb.newValue(typ, OwnershipCompilerTemporary, op.Location)
	op.Results = []Value{value}
	fb.emit(op)
	return builtValue{id: value.ID, typ: typ}
}

// lowerTemporaryBorrow materializes the call-lifetime Place of a fresh owned
// argument or receiver and borrows it (MD-076; borrowing.md § 5.4): the
// producing expression is evaluated once, a shared borrow addresses a
// read-only value root over its result and a mutable borrow an address-taken
// compiler storage initialized with it. Only trivially destructible
// temporaries lower here; full-expression cleanup of others belongs to P17.
//
// Rules:
//   - rules/memory/borrowing.md — § 5.4
//   - rules/memory/destruction.md — § 8(7)
//   - rules/corrections/applied/md076-call-lifetime-temporary-borrows-correction-20261021.md — § 9
func (fb *functionBuilder) lowerTemporaryBorrow(expr ast.Expression, plan sema.ResolvedReferencePlan) (builtValue, error) {
	token := expressionToken(expr)
	if !sema.TriviallyDestructible(plan.Place.Type) {
		return builtValue{}, fb.unsupported("borrowed temporary that needs full-expression cleanup", token)
	}
	element, err := fb.owner.internType(plan.Place.Type)
	if err != nil {
		return builtValue{}, err
	}
	previous := fb.temporarySource
	fb.temporarySource = expr
	value, err := fb.buildExpr(expr, element)
	fb.temporarySource = previous
	if err != nil {
		return builtValue{}, err
	}
	if value.typ != element || !fb.storageAllowed(value.typ) {
		return builtValue{}, fb.unsupported("borrowed temporary of a non-trivial type", token)
	}
	loc := location(token)
	mutable := plan.Facts.MutableAuthority
	placeFacts := placeMetadataOf(plan.Place)
	var place builtValue
	if mutable {
		id := fb.nextStorage
		fb.nextStorage++
		fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Name: "temporary", Type: element, Mutable: true, Class: StorageLocalAutomatic, Location: loc, AddressTaken: true})
		fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: loc})
		fb.emit(Operation{Kind: OpStorageInit, Storage: id, Operands: []ValueID{value.id}, Location: loc})
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: element, Mutable: true})
		place = fb.placeResult(Operation{Kind: OpPlaceStorageRoot, Storage: id, Place: placeFacts, Location: loc}, placeType)
	} else {
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: element})
		place = fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{value.id}, Place: placeFacts, Location: loc}, placeType)
	}
	borrow := &ast.RefExpression{Token: token, Mutable: mutable, Value: expr}
	return fb.emitBorrow(borrow, plan, place)
}
