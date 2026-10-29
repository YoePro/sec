package semantic

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// maxUnrolledMembershipLength bounds the element tests a fixed-array
// membership expands to; longer arrays stay an explicit capability boundary.
const maxUnrolledMembershipLength = 64

// buildFixedArrayMembership lowers `value in array` and `value not in array`
// for a fixed array of built-in integers. The value is evaluated first and the
// array second, each exactly once; the elements are then compared in index
// order with constant, proven-safe extracts, and the first equal element ends
// the search. `not in` negates the completed result.
//
// Rules:
//   - rules/foundations/operators.md — "Membership expression", "Array and slice membership"
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 42, 48, 82–83
func (fb *functionBuilder) buildFixedArrayMembership(expr *ast.InfixExpression, resolved sema.ResolvedOperator) (builtValue, bool, error) {
	if resolved.RightType == nil {
		return builtValue{}, false, nil
	}
	arraySource := *resolved.RightType
	length, fixed := sema.FixedArrayLength(arraySource)
	if !fixed || arraySource.Element == nil || !length.IsInt64() || length.Int64() > maxUnrolledMembershipLength {
		return builtValue{}, false, nil
	}
	arrayType, err := fb.owner.internType(arraySource)
	if err != nil {
		return builtValue{}, true, err
	}
	elementType, err := fb.owner.internType(*arraySource.Element)
	if err != nil {
		return builtValue{}, true, err
	}
	if !isBuiltinIntegerType(fb.owner.module.Types, elementType) {
		return builtValue{}, false, nil
	}
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, true, err
	}
	indexType, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return builtValue{}, true, err
	}
	loc := location(expr.Token)
	value, err := fb.buildExpr(expr.Left, elementType)
	if err != nil {
		return builtValue{}, true, err
	}
	array, err := fb.buildExpr(expr.Right, arrayType)
	if err != nil {
		return builtValue{}, true, err
	}
	if value.typ != elementType || array.typ != arrayType {
		return builtValue{}, true, fb.unsupported("fixed-array membership operand type mismatch", expr.Token)
	}
	var result builtValue
	count := int(length.Int64())
	if count == 0 {
		absent := false
		result = fb.result(Operation{Kind: OpConstBool, Bool: &absent, Location: loc}, boolType)
	} else {
		merge := fb.newBlock()
		found := fb.newValue(boolType, OwnershipImmediate, loc)
		merge.Parameters = []Value{found}
		for index := 0; index < count; index++ {
			position := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(int64(index)), Location: loc}, indexType)
			element := fb.result(Operation{
				Kind: OpArrayExtract, Operands: []ValueID{array.id, position.id},
				ArrayActions:   []ArrayTransferAction{ArrayActionCopyTrivial},
				ArrayCheckKind: ArrayIndexProvenSafe, ArrayProofKind: ArrayIndexProofConstant, Location: loc,
			}, elementType)
			equal := fb.result(Operation{Kind: OpIntCompare, IntegerCompare: IntegerCompareEQ, Operands: []ValueID{element.id, value.id}, Operator: expr.Operator, Location: loc}, boolType)
			if index == count-1 {
				fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{equal.id}}}, Operator: expr.Operator, Location: loc})
				break
			}
			next := fb.newBlock()
			fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{equal.id}, Successors: []BranchTarget{
				{Block: merge.ID, Arguments: []ValueID{equal.id}}, {Block: next.ID},
			}, Operator: expr.Operator, Location: loc})
			fb.current = next
		}
		fb.current = merge
		result = builtValue{id: found.ID, typ: boolType}
	}
	if resolved.Kind == sema.ResolvedNegatedMembershipCompare {
		return fb.result(Operation{Kind: OpBoolNot, Operands: []ValueID{result.id}, Operator: expr.Operator, Location: loc}, boolType), true, nil
	}
	return result, true, nil
}
