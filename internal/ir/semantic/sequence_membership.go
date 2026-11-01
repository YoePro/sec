package semantic

import (
	"math/big"
	"sec/internal/ast"
	"sec/internal/sema"
)

// membershipScalarType admits only equality operations already represented by
// this track. Strings, decimals and aggregate equality retain their named gates.
// Rules: rules/foundations/operators.md — "Array and slice membership", §A.14.
func membershipScalarType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	if !ok {
		return false
	}
	switch typ.Kind {
	case TypeInt, TypeUint, TypeByte, TypeChar, TypeRune, TypeBool, TypeEnum, TypeFloat:
		return true
	}
	return false
}

// membershipScalarEquality reuses ordinary equality: float EQ is ordered
// (NaN never matches), enums retain nominal identity, and bool uses bool flow.
// Rules: rules/foundations/operators.md — "Equality", "Boolean equality", §A.14;
// rules/mlir/packages/sec-mlir-dialect_package20.md — §24.
func (fb *functionBuilder) membershipScalarEquality(left, right builtValue, boolean TypeID, loc Location) builtValue {
	typ, _ := fb.owner.module.Types.Lookup(left.typ)
	kind := OpIntCompare
	switch typ.Kind {
	case TypeFloat:
		kind = OpFloatCompare
	case TypeEnum:
		kind = OpEnumCompare
	case TypeBool:
		invert, merge := fb.newBlock(), fb.newBlock()
		result := fb.newValue(boolean, OwnershipImmediate, loc)
		merge.Parameters = []Value{result}
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{left.id}, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{right.id}}, {Block: invert.ID}}, Location: loc})
		fb.current = invert
		negated := fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{right.id}, Location: loc}, boolean)
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{negated.id}}}, Location: loc})
		fb.current = merge
		return builtValue{id: result.ID, typ: boolean}
	}
	return fb.result(Operation{Kind: kind, IntegerCompare: IntegerCompareEQ, Operands: []ValueID{left.id, right.id}, Operator: "==", Location: loc}, boolean)
}

// buildSequenceMembership builds constant-size search CFG over a long fixed
// array or a P16 slice. Operands run left-to-right once; the first match exits.
// Reads preserve the holder and authority. Standard temporal/spatial guards
// protect each read; no new unchecked range proof or allocation is invented.
// The uint increment cannot overflow on the i < length continuation, and still
// uses the track's checked arithmetic instead of an unchecked arithmetic op.
// Rules: rules/foundations/operators.md — "Array and slice membership", §A.14;
// rules/mlir/packages/sec-mlir-dialect_package14.md — §§31–41,82–83;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45–58,63.
func (fb *functionBuilder) buildSequenceMembership(expr *ast.InfixExpression, resolved sema.ResolvedOperator) (builtValue, bool, error) {
	if resolved.RightType == nil {
		return builtValue{}, false, nil
	}
	sourceType := *resolved.RightType
	slice := sourceSliceType(sourceType)
	elementSource := sourceType.Element
	if slice && sourceType.Kind == sema.ReferenceType {
		elementSource = sourceType.Element.Element
	}
	if elementSource == nil {
		return builtValue{}, false, nil
	}
	element, err := fb.owner.internType(*elementSource)
	if err != nil {
		return builtValue{}, true, err
	}
	if !membershipScalarType(fb.owner.module.Types, element) {
		return builtValue{}, false, nil
	}
	boolean, err := fb.boolType()
	if err != nil {
		return builtValue{}, true, err
	}
	indexType, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return builtValue{}, true, err
	}
	loc := location(expr.Token)
	needle, err := fb.buildExpr(expr.Left, element)
	if err != nil {
		return builtValue{}, true, err
	}
	if needle.typ != element {
		return builtValue{}, true, fb.unsupported("sequence membership needle type mismatch", expr.Token)
	}
	var source, length builtValue
	var facts *ReferenceMetadata
	var validity ValueID
	if slice {
		source, facts, err = fb.sliceSourceValue(expr.Right)
		if err != nil {
			return builtValue{}, true, err
		}
		validity, err = fb.validateReference(source, facts, loc)
		if err != nil {
			return builtValue{}, true, err
		}
		length = fb.result(Operation{Kind: OpSliceLength, Operands: []ValueID{source.id}, ReferenceGuard: validity, Location: loc}, indexType)
	} else {
		arrayType, e := fb.owner.internType(sourceType)
		if e != nil {
			return builtValue{}, true, e
		}
		source, err = fb.buildExpr(expr.Right, arrayType)
		if err != nil {
			return builtValue{}, true, err
		}
		count, _ := sema.FixedArrayLength(sourceType)
		length = fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Set(count), Location: loc}, indexType)
	}
	definition, known := fb.owner.module.Types.Lookup(source.typ)
	if !known || definition.Element != element || slice && definition.Kind != TypeSharedSlice && definition.Kind != TypeMutableSlice || !slice && definition.Kind != TypeArray {
		return builtValue{}, true, fb.unsupported("sequence membership source type mismatch", expr.Token)
	}
	zero := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, indexType)
	absent, present := false, true
	no := fb.result(Operation{Kind: OpConstBool, Bool: &absent, Location: loc}, boolean)
	yes := fb.result(Operation{Kind: OpConstBool, Bool: &present, Location: loc}, boolean)
	header, body, next, merge := fb.newBlock(), fb.newBlock(), fb.newBlock(), fb.newBlock()
	index := fb.newValue(indexType, OwnershipImmediate, loc)
	header.Parameters = []Value{index}
	result := fb.newValue(boolean, OwnershipImmediate, loc)
	merge.Parameters = []Value{result}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: header.ID, Arguments: []ValueID{zero.id}}}, Location: loc})
	fb.current = header
	condition := fb.forCompare(IntegerCompareLT, builtValue{id: index.ID, typ: indexType}, length, boolean, loc)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: body.ID}, {Block: merge.ID, Arguments: []ValueID{no.id}}}, Location: loc})
	fb.current = body
	predicateKind, operation := OpArrayIndexInBounds, "fixed-array-index"
	if slice {
		predicateKind, operation = OpSliceIndexInBounds, "slice-index"
	}
	guard := fb.result(Operation{Kind: predicateKind, Operands: []ValueID{source.id, index.ID}, ReferenceGuard: validity, Location: loc}, boolean)
	read, failure := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{guard.id}, Successors: []BranchTarget{{Block: read.ID}, {Block: failure.ID}}, Location: loc})
	fb.current = failure
	fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: operation, Location: loc})
	fb.current = read
	var value builtValue
	access := Operation{Operands: []ValueID{source.id, index.ID}, ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: guard.id, Location: loc}
	if slice {
		access.Kind, access.Reference, access.ReferenceGuard = OpSliceElementPlace, cloneReferenceMetadata(facts), validity
		sourceDefinition, _ := fb.owner.module.Types.Lookup(source.typ)
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: element, Mutable: sourceDefinition.Kind == TypeMutableSlice})
		place := fb.placeResult(access, placeType)
		value = fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: loc}, element)
	} else {
		access.Kind, access.ArrayActions = OpArrayExtract, []ArrayTransferAction{ArrayActionCopyTrivial}
		value = fb.result(access, element)
	}
	equal := fb.membershipScalarEquality(needle, value, boolean, loc)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{equal.id}, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{yes.id}}, {Block: next.ID}}, Location: loc})
	fb.current = next
	one := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(1), Location: loc}, indexType)
	increment, err := fb.forAdd(builtValue{id: index.ID, typ: indexType}, one, indexType, loc)
	if err != nil {
		return builtValue{}, true, err
	}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: header.ID, Arguments: []ValueID{increment.id}}}, Location: loc})
	fb.current = merge
	found := builtValue{id: result.ID, typ: boolean}
	if slice {
		if _, temporary := expr.Right.(*ast.RefExpression); temporary {
			if _, err := fb.discardSliceHolder(source, expressionToken(expr.Right)); err != nil {
				return builtValue{}, true, err
			}
		}
	}
	if resolved.Kind == sema.ResolvedNegatedMembershipCompare {
		found = fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{found.id}, Location: loc}, boolean)
	}
	return found, true, nil
}
