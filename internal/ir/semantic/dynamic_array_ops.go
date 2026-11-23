package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// Package 18 dynamic-array operations in Semantic IR.
//
//   - dynamic-array.empty: the canonical non-allocating empty owner, length
//     zero, no backing, no element constructed (P18 §§ 7, 22, 80, 83);
//   - dynamic-array.length: the runtime length as uint, observing the owner
//     through its Place without moving or copying it (§§ 13, 49, 85);
//   - dynamic-array.index-in-bounds: the total predicate
//     0 <= index < runtime length with P14/P16 signedness (§ 75);
//   - dynamic-array.element-place: the element Place of a guarded index,
//     with the owner's authority and its current backing-domain dependency;
//     no backing pointer arithmetic is exposed (§§ 74, 76, 77).
const (
	OpDynamicArrayEmpty         OpKind = "dynamic-array.empty"
	OpDynamicArrayLength        OpKind = "dynamic-array.length"
	OpDynamicArrayIndexInBounds OpKind = "dynamic-array.index-in-bounds"
	OpDynamicArrayElementPlace  OpKind = "dynamic-array.element-place"
)

// verifyDynamicArrayOperation checks the operation shapes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 7, 13, 49, 52, 74–77, 80, 83, 85.
func verifyDynamicArrayOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	switch op.Kind {
	case OpDynamicArrayEmpty:
		if len(op.Operands) != 0 || len(op.Results) != 1 || len(op.Successors) != 0 {
			return fmt.Errorf("dynamic-array.empty takes no operands and has one result")
		}
		result, ok := types.Lookup(op.Results[0].Type)
		if !ok || result.Kind != TypeDynamicArray {
			return fmt.Errorf("dynamic-array.empty must produce a dynamic array")
		}
		if op.Results[0].Ownership != OwnershipOwned {
			return fmt.Errorf("dynamic-array.empty produces an owned value")
		}
	case OpDynamicArrayLength:
		if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.Successors) != 0 {
			return fmt.Errorf("dynamic-array.length requires one owner Place and one uint result")
		}
		place, ok := types.Lookup(values[op.Operands[0]].Type)
		if !ok || place.Kind != TypePlace {
			return fmt.Errorf("dynamic-array.length observes the owner through a Place, never by moving it")
		}
		owner, ok := types.Lookup(place.Element)
		if !ok || owner.Kind != TypeDynamicArray {
			return fmt.Errorf("dynamic-array.length requires a Place of a dynamic array")
		}
		result, ok := types.Lookup(op.Results[0].Type)
		if !ok || result.Kind != TypeUint || result.Name != "uint" || !result.TargetSize {
			return fmt.Errorf("dynamic-array.length produces the target-sized uint length")
		}
	case OpDynamicArrayIndexInBounds, OpDynamicArrayElementPlace:
		if len(op.Operands) != 2 || len(op.Results) != 1 || len(op.Successors) != 0 {
			return fmt.Errorf("%s requires an owner Place, an index and one result", op.Kind)
		}
		owner, ok := types.Lookup(values[op.Operands[0]].Type)
		if !ok || owner.Kind != TypePlace {
			return fmt.Errorf("%s addresses the owner through a Place", op.Kind)
		}
		array, ok := types.Lookup(owner.Element)
		if !ok || array.Kind != TypeDynamicArray {
			return fmt.Errorf("%s requires a Place of a dynamic array", op.Kind)
		}
		if !isBuiltinIntegerType(types, values[op.Operands[1]].Type) {
			return fmt.Errorf("%s requires a built-in integer index", op.Kind)
		}
		if op.Kind == OpDynamicArrayIndexInBounds {
			if !typeHasKind(types, op.Results[0].Type, TypeBool) {
				return fmt.Errorf("dynamic-array.index-in-bounds produces bool")
			}
			return nil
		}
		place, ok := types.Lookup(op.Results[0].Type)
		if !ok || place.Kind != TypePlace || place.Element != array.Element || place.Mutable != owner.Mutable {
			return fmt.Errorf("dynamic-array.element-place must yield an element Place with the owner's authority")
		}
		if err := verifyArrayBoundsMetadata(op); err != nil {
			return err
		}
		// A runtime length admits only a guard or a branch/analysis proof of
		// `index < Len` (package18 § 41); no constant or range proof exists.
		if op.ArrayCheckKind == ArrayIndexProvenSafe && op.ArrayProofKind != ArrayIndexProofBranch && op.ArrayProofKind != ArrayIndexProofAnalysis {
			return fmt.Errorf("dynamic-array.element-place proof %q cannot bound a runtime length", op.ArrayProofKind)
		}
		if op.Place == nil || op.Place.BackingDomain == 0 {
			return fmt.Errorf("dynamic-array.element-place must carry the owner's backing-domain dependency")
		}
	}
	return nil
}

// buildDynamicArrayEmpty emits the empty owner.
func (fb *functionBuilder) buildDynamicArrayEmpty(typeID TypeID, loc Location) builtValue {
	value := fb.newValue(typeID, OwnershipOwned, loc)
	fb.emit(Operation{Kind: OpDynamicArrayEmpty, Results: []Value{value}, Location: loc})
	return builtValue{id: value.ID, typ: typeID}
}

// buildDynamicArrayLength lowers `values.Len` and `len(values)` for an owning
// `T[]`: both are the same uint count (MD-074), read through the owner's
// Place so the owner is neither moved nor copied.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 13, 14, 49, 50.
func (fb *functionBuilder) buildDynamicArrayLength(source ast.Expression, token lexer.Token) (builtValue, error) {
	place, err := fb.dynamicArrayOwnerPlace(source)
	if err != nil {
		return builtValue{}, err
	}
	uintType, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpDynamicArrayLength, Operands: []ValueID{place.id}, Location: location(token)}, uintType), nil
}

// dynamicArrayOwnerPlace is the Place of a dynamic-array owner held by a
// binding: its owned storage, a defer capture, or the read-only root of an
// SSA value. Other owners are a named boundary.
func (fb *functionBuilder) dynamicArrayOwnerPlace(source ast.Expression) (builtValue, error) {
	if member, ok := source.(*ast.MemberExpression); ok {
		return fb.dynamicArrayFieldOwnerPlace(member)
	}
	identifier, ok := source.(*ast.Identifier)
	if !ok {
		return builtValue{}, fb.unsupported("dynamic-array observation of a non-binding owner (P18-23)", expressionToken(source))
	}
	place, err := fb.dynamicArrayBindingPlace(identifier)
	if err != nil {
		return builtValue{}, err
	}
	if typ, _ := fb.owner.module.Types.Lookup(place.typ); typ.Kind != TypePlace {
		return builtValue{}, fmt.Errorf("binding root of %s is not a Place", identifier.Value)
	} else if element, _ := fb.owner.module.Types.Lookup(typ.Element); element.Kind != TypeDynamicArray {
		return builtValue{}, fb.unsupported("dynamic-array owner reached through a reference (P18-23)", identifier.Token)
	}
	return place, nil
}

// dynamicArrayFieldOwnerPlace is the Place of a dynamic-array owner stored in
// a field path below a binding (`bag.items`, `outer.inner.items`): the
// binding's root Place projected through the resolved stored fields with the
// root's authority. Other paths remain named boundaries.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 74, 103, 131.
func (fb *functionBuilder) dynamicArrayFieldOwnerPlace(member *ast.MemberExpression) (builtValue, error) {
	steps := []ast.Expression{}
	var current ast.Expression = member
	for {
		step, ok := current.(*ast.MemberExpression)
		if !ok {
			break
		}
		steps = append([]ast.Expression{step}, steps...)
		current = step.Object
	}
	root, ok := current.(*ast.Identifier)
	if !ok {
		return builtValue{}, fb.unsupported("dynamic-array owner below a non-binding root (P18-23)", member.Token)
	}
	resolved, ok := fb.owner.analyzer.ResolvedPlaceOf(member)
	if !ok || len(resolved.Projections) != len(steps) {
		return builtValue{}, fb.unsupported("dynamic-array field owner without a resolved Place (P18-23)", member.Token)
	}
	place, err := fb.dynamicArrayBindingPlace(root)
	if err != nil {
		return builtValue{}, err
	}
	rootType, _ := fb.owner.module.Types.Lookup(place.typ)
	if rootType.Kind != TypePlace {
		return builtValue{}, fmt.Errorf("binding root of %s is not a Place", root.Value)
	}
	facts := placeMetadataOf(resolved)
	place, err = fb.projectPlace(steps, resolved.Projections, place, rootType.Mutable, facts)
	if err != nil {
		return builtValue{}, err
	}
	owner, _ := fb.owner.module.Types.Lookup(place.typ)
	if element, _ := fb.owner.module.Types.Lookup(owner.Element); element.Kind != TypeDynamicArray {
		return builtValue{}, fb.unsupported("field path that is not a dynamic-array owner (P18-23)", member.Token)
	}
	return place, nil
}

// dynamicArrayBindingPlace is the Place of a binding's value: its owned storage, a
// defer capture, mutable storage, or the read-only root of an SSA value.
func (fb *functionBuilder) dynamicArrayBindingPlace(identifier *ast.Identifier) (builtValue, error) {
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return builtValue{}, fmt.Errorf("missing resolved binding for %s", identifier.Value)
	}
	bound, ok := fb.bindings[fact.ID]
	if !ok {
		return builtValue{}, fmt.Errorf("binding %d has no IR value", fact.ID)
	}
	if typ, _ := fb.owner.module.Types.Lookup(bound.typ); typ.Kind == TypeReference {
		return builtValue{}, fb.unsupported("dynamic-array owner reached through a reference (P18-23)", identifier.Token)
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
		return builtValue{id: bound.capture, typ: fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ, Mutable: bound.mutable})}, nil
	case fb.isOwnedStorage(fact.ID):
		return fb.ownedStoragePlace(fb.ownedStorages[fact.ID], identifier.Token), nil
	case bound.storage != 0:
		fb.markStorageAddressTaken(bound.storage)
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ, Mutable: true})
		return fb.placeResult(Operation{Kind: OpPlaceStorageRoot, Storage: bound.storage, Place: facts, Location: loc}, placeType), nil
	default:
		// An immutable owner is an SSA value; ValueID 0 is a valid value.
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: bound.typ})
		if facts == nil {
			return builtValue{}, fb.unsupported("dynamic-array owner without a resolved Place", identifier.Token)
		}
		return fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{bound.value}, Place: facts, Location: loc}, placeType), nil
	}
}

func (fb *functionBuilder) isOwnedStorage(id sema.BindingID) bool {
	_, owned := fb.ownedStorages[id]
	return owned
}

// lowerDynamicArrayElementPlace evaluates the owner Place and then the index
// once, branches on dynamic-array.index-in-bounds and forms the element
// Place only on the successful edge. Failure is the ordinary bounds panic or
// IndexError under a resolved try. The Place depends on the owner's current
// backing domain; an empty owner has no valid index (§ 79).
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 40–41, 74–79.
func (fb *functionBuilder) lowerDynamicArrayElementPlace(expr *ast.IndexExpression, plan sema.ResolvedDynamicArrayIndexPlan) (builtValue, error) {
	if plan.Owner == nil || plan.Owner.BackingDomain == 0 {
		return builtValue{}, fb.unsupported("dynamic-array index of an owner without a published Place (P18-23)", expr.Token)
	}
	proven := plan.CheckKind == sema.ArrayIndexProvenSafe
	if !proven && plan.CheckKind != sema.ArrayIndexRuntimeCheck {
		return builtValue{}, fmt.Errorf("dynamic-array index has check kind %q", plan.CheckKind)
	}
	owner, err := fb.dynamicArrayOwnerPlace(expr.Left)
	if err != nil {
		return builtValue{}, err
	}
	element, err := fb.owner.internType(plan.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	indexType, err := fb.owner.internType(plan.IndexType)
	if err != nil {
		return builtValue{}, err
	}
	index, err := fb.buildExpr(expr.Index, indexType)
	if err != nil {
		return builtValue{}, err
	}
	if index.typ != indexType {
		return builtValue{}, fmt.Errorf("dynamic-array index operand differs from its resolved integer type")
	}
	loc := location(expr.Token)
	ownerType, _ := fb.owner.module.Types.Lookup(owner.typ)
	facts := placeMetadataOf(plan.Owner.Place)
	facts.BackingDomain = uint32(plan.Owner.BackingDomain)
	elementPlaceType := func() TypeID {
		return fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: element, Mutable: ownerType.Mutable})
	}
	if proven {
		// § 41: Sema proved 0 <= index < Len; the Place needs no guard.
		proof, valid := semanticArrayIndexProof(plan.ProofKind)
		if !valid || plan.FailureMode != sema.ArrayIndexFailureNone {
			return builtValue{}, fmt.Errorf("proven dynamic-array index without a branch/analysis proof")
		}
		op := Operation{
			Kind: OpDynamicArrayElementPlace, Operands: []ValueID{owner.id, index.id}, ArrayIndexSigned: plan.IndexSigned,
			ArrayCheckKind: ArrayIndexProvenSafe, ArrayProofKind: proof,
			Integer: cloneBigInt(plan.ConstantIndex), Place: facts, Location: loc,
		}
		return fb.placeResult(op, elementPlaceType()), nil
	}
	_, protected := fb.failurePoint(expr)
	if protected != (plan.FailureMode == sema.ArrayIndexFailureFallible) || !protected && plan.FailureMode != sema.ArrayIndexFailureOrdinary {
		return builtValue{}, fb.unsupported("dynamic-array index failure mode without its resolved try boundary", expr.Token)
	}
	boolean := fb.owner.module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
	operands := []ValueID{owner.id, index.id}
	predicate := fb.result(Operation{Kind: OpDynamicArrayIndexInBounds, Operands: operands, ArrayIndexSigned: plan.IndexSigned, Location: loc}, boolean)
	success, failure := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{predicate.id}, Successors: []BranchTarget{{Block: success.ID}, {Block: failure.ID}}, Location: loc})
	fb.current = failure
	if protected {
		errorValue, err := fb.outOfBoundsError(plan.ErrorType, expr.Token)
		if err != nil {
			return builtValue{}, err
		}
		fb.routeFailure(expr, errorValue, loc)
	} else {
		fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: plan.PanicOperation, Location: loc})
	}
	fb.current = success
	op := Operation{
		Kind: OpDynamicArrayElementPlace, Operands: operands, ArrayIndexSigned: plan.IndexSigned,
		ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: predicate.id,
		Integer: cloneBigInt(plan.ConstantIndex), Place: facts, Location: loc,
	}
	return fb.placeResult(op, elementPlaceType()), nil
}

// lowerDynamicArrayIndexRead reads a trivially copyable element through its
// guarded Place; the owner is neither moved nor copied (§ 43).
func (fb *functionBuilder) lowerDynamicArrayIndexRead(expr *ast.IndexExpression, plan sema.ResolvedDynamicArrayIndexPlan) (builtValue, error) {
	if plan.UseKind != sema.ArrayIndexRead || plan.Action != sema.ArrayTransferCopyTrivial {
		return builtValue{}, fb.unsupported("non-trivial dynamic-array element transfer or non-read use", expr.Token)
	}
	place, err := fb.lowerDynamicArrayElementPlace(expr, plan)
	if err != nil {
		return builtValue{}, err
	}
	typ, _ := fb.owner.module.Types.Lookup(place.typ)
	if !package14TrivialValue(fb.owner.module, typ.Element) {
		return builtValue{}, fb.unsupported("dynamic-array element read outside the trivial value surface", expr.Token)
	}
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: location(expr.Token)}, typ.Element), nil
}

// lowerDynamicArrayIndexWrite establishes the guarded destination Place
// before evaluating the RHS and replaces a trivial element through
// place.write; a compound write reads that same Place once.
// Rules: rules/foundations/operators.md — "Assignment evaluation order", "Compound assignment";
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 42, 74–76.
func (fb *functionBuilder) lowerDynamicArrayIndexWrite(stmt *ast.AssignmentStatement, index *ast.IndexExpression, plan sema.ResolvedDynamicArrayIndexPlan) error {
	if plan.UseKind != sema.ArrayIndexWrite || plan.Action != sema.ArrayTransferConstructDirect || !fb.owner.triviallyDestructible(plan.ElementType) || fb.owner.copyClassificationOf(plan.ElementType) != sema.CopyTrivial {
		return fb.unsupported("non-trivial dynamic-array element replacement", stmt.Token)
	}
	place, err := fb.lowerDynamicArrayElementPlace(index, plan)
	if err != nil {
		return err
	}
	typ, _ := fb.owner.module.Types.Lookup(place.typ)
	if !typ.Mutable {
		return fmt.Errorf("dynamic-array element write through a read-only owner Place")
	}
	if !package14TrivialValue(fb.owner.module, typ.Element) {
		return fb.unsupported("dynamic-array element replacement outside the trivial value surface", stmt.Token)
	}
	var value builtValue
	if stmt.Operator == "=" {
		value, err = fb.buildExpr(stmt.Value, typ.Element)
	} else {
		resolved, ok := fb.owner.analyzer.ResolvedCompoundAssignmentOf(stmt)
		if !ok {
			return fb.unsupported("dynamic-array compound assignment without a resolved integer operator", stmt.Token)
		}
		current := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: location(stmt.Token)}, typ.Element)
		value, err = fb.buildCompoundValue(stmt, resolved, current)
	}
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: location(stmt.Token)})
	return nil
}

// dynamicArrayElementBorrowPlace is the guarded element Place a direct
// `ref values[i]` or `ref mut values[i]` borrows. The Place names the
// owner's backing domain; Sema rejects every growth of the owner while the
// borrow lives (S1162), so the reference depends on the current backing
// incarnation (§ 77). Deeper paths through an element stay a boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 38, 48, 74–77.
func (fb *functionBuilder) dynamicArrayElementBorrowPlace(reference *ast.RefExpression, index *ast.IndexExpression, plan sema.ResolvedDynamicArrayIndexPlan, steps int) (builtValue, error) {
	if steps != 1 || reference.Value != ast.Expression(index) {
		return builtValue{}, fb.unsupported("Place path through a dynamic-array element (P18-23)", index.Token)
	}
	use := sema.ArrayIndexBorrow
	if reference.Mutable {
		use = sema.ArrayIndexMutBorrow
	}
	if plan.UseKind != use || !plan.EpochDependent {
		return builtValue{}, fmt.Errorf("dynamic-array element borrow differs from its resolved use")
	}
	place, err := fb.lowerDynamicArrayElementPlace(index, plan)
	if err != nil {
		return builtValue{}, err
	}
	if typ, _ := fb.owner.module.Types.Lookup(place.typ); reference.Mutable && !typ.Mutable {
		return builtValue{}, fb.unsupported("mutable borrow of a read-only Place", reference.Token)
	}
	return place, nil
}

// receiverIsDynamicArray reports whether a member receiver is an owning T[].
func (fb *functionBuilder) receiverIsDynamicArray(receiver ast.Expression) bool {
	typ, ok := fb.owner.analyzer.ResolvedTypeOf(receiver)
	if !ok {
		return false
	}
	_, dynamic := sema.ResolveDynamicArrayPlan(typ)
	return dynamic
}

// buildDynamicArrayIsEmpty lowers `values.IsEmpty` for an owning T[] as
// `length == 0`, observing the owner through its Place without moving it.
// Rules: rules/collections/collections.md — § 3.2 "IsEmpty";
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 49, 85, 131.
func (fb *functionBuilder) buildDynamicArrayIsEmpty(expr *ast.MemberExpression) (builtValue, error) {
	length, err := fb.buildDynamicArrayLength(expr.Object, expr.Property.Token)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(expr.Property.Token)
	zero := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, length.typ)
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpIntCompare, IntegerCompare: IntegerCompareEQ, Operands: []ValueID{length.id, zero.id}, Location: loc}, boolType), nil
}

// dynamicArrayMemberBoundary names the owning-array members schema 14 cannot
// represent: every method other than Append (planned growth) that has no
// resolved user target, such as Clear and RemoveAt, which remove elements or
// shrink the length; ToString keeps its P23 string boundary. No schema-14 operation does either (P18 § 81), so the
// call stops at a named boundary instead of an unresolved call.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 81, 131–132.
func (fb *functionBuilder) dynamicArrayMemberBoundary(call *ast.CallExpression) (error, bool) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil || fb.owner.maxPackage < dynamicArrayLoweringPackage || !fb.receiverIsDynamicArray(member.Object) {
		return nil, false
	}
	if member.Property.Value == "ToString" {
		// String materialization keeps its own P23 boundary.
		return nil, false
	}
	if _, planned := fb.owner.analyzer.ResolvedAllocationPlanOf(call); planned {
		return nil, false
	}
	if _, resolved := fb.owner.analyzer.ResolvedCallTarget(call); resolved {
		return nil, false
	}
	return fb.unsupported("dynamic-array "+member.Property.Value+" (no schema-14 operation removes elements or shrinks the length)", member.Property.Token), true
}

// coreArrayMaterializationBoundary names the core byte[]/rune[] producers
// Semantic IR cannot lower: ToByteArray needs a byte read of the string
// payload (P23), and ToRuneArray without a materialization plan has a
// temporary string receiver; a planned ToRuneArray lowers instead
// (rune_array_materialization.go; P18 §§ 72–73, 133).
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 72–73, 133.
func (fb *functionBuilder) coreArrayMaterializationBoundary(call *ast.CallExpression) (error, bool) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil || (member.Property.Value != "ToByteArray" && member.Property.Value != "ToRuneArray") {
		return nil, false
	}
	receiver, ok := fb.owner.analyzer.ResolvedTypeOf(member.Object)
	if !ok || dereferenceSemaType(receiver).Kind != sema.StringType {
		return nil, false
	}
	if member.Property.Value == "ToRuneArray" {
		return fb.unsupported("string.ToRuneArray of a string that is not a Place (a temporary string value needs P23 string lowering)", member.Property.Token), true
	}
	return fb.unsupported("string.ToByteArray materialization (no Semantic IR operation reads UTF-8 bytes before P23 string lowering)", member.Property.Token), true
}

func dereferenceSemaType(typ sema.Type) sema.Type {
	for typ.Kind == sema.ReferenceType && typ.Element != nil {
		typ = *typ.Element
	}
	return typ
}

// verifyDynamicArrayElementBorrows checks that a direct borrow of a guarded
// dynamic-array element Place names that Place's backing domain as its
// invalidation domain: the reference depends on the owner's current backing
// incarnation (package18 §§ 77, 100).
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 77, 100.
func verifyDynamicArrayElementBorrows(fn *Function) error {
	backing := map[ValueID]uint32{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpDynamicArrayElementPlace && op.Place != nil && len(op.Results) == 1 {
				backing[op.Results[0].ID] = op.Place.BackingDomain
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpReferenceBorrowShared && op.Kind != OpReferenceBorrowMutable || len(op.Operands) == 0 || op.Reference == nil {
				continue
			}
			if domain, element := backing[op.Operands[0]]; element && op.Reference.InvalidationDomain != domain {
				return fmt.Errorf("%s of a dynamic-array element in ^%d names invalidation domain #%d, not the element Place's backing domain #%d", op.Kind, block.ID, op.Reference.InvalidationDomain, domain)
			}
		}
	}
	return nil
}
