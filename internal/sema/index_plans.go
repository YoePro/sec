package sema

import (
	"math/big"
	"sec/internal/ast"
)

// recordListIndexPlan publishes the runtime-checked frontend decision for a
// list element Place. It deliberately does not create an array-index fact:
// Semantic IR must reject list lowering until it has dedicated operations.
//
// Rules:
//   - rules/collections/collections.md — §8 "Indexing"
func (a *Analyzer) recordListIndexPlan(expr *ast.IndexExpression, listType, elementType, indexType Type, use ArrayIndexUseKind) {
	plan := ResolvedListIndexPlan{
		ListType:    listType,
		ElementType: elementType,
		IndexType:   indexType,
		IndexSigned: indexType.Kind == IntType,
		CheckKind:   ArrayIndexRuntimeCheck,
		UseKind:     use,
		Action:      arrayIndexTransferAction(elementType, use),
		FailureMode: ArrayIndexFailureOrdinary,
		ErrorType:   a.indexErrorType(),
	}
	if constant, ok := a.integerConstantValue(expr.Index); ok {
		plan.ConstantIndex = new(big.Int).Set(constant)
	}
	a.recordResolvedListIndexPlan(expr, plan)
	a.recordListIndexEffect(expr, plan)
}

// recordFixedArrayIndexPlan publishes the already-resolved index decision
// required by SEC-MLIR Package 14 sections 31-36. It preserves the source
// integer type and uses exact constants/type ranges as proof provenance.
func (a *Analyzer) recordFixedArrayIndexPlan(expr *ast.IndexExpression, arrayType, elementType, indexType Type, use ArrayIndexUseKind) {
	length, ok := exactFixedArrayLength(arrayType)
	if !ok {
		return
	}
	plan := ResolvedArrayIndexPlan{
		ArrayType:   arrayType,
		ArrayLength: length,
		ElementType: elementType,
		IndexType:   indexType,
		IndexSigned: indexType.Kind == IntType,
		CheckKind:   ArrayIndexRuntimeCheck,
		UseKind:     use,
		Action:      arrayIndexTransferAction(elementType, use),
		FailureMode: ArrayIndexFailureOrdinary,
		ErrorType:   a.indexErrorType(),
	}
	plan.ConstantIndex, plan.ProofKind = a.sequenceIndexProof(expr, indexType, length)
	if plan.ProofKind != "" {
		plan.CheckKind = ArrayIndexProvenSafe
		plan.FailureMode = ArrayIndexFailureNone
		plan.ErrorType = Type{}
	}

	a.recordResolvedArrayIndexPlan(expr, plan)
	a.recordArrayIndexEffect(expr, plan)
}

// refinementProvesArrayIndex combines currently dominating branch/assertion
// conditions into the exact 0 <= index < N proof required for a fixed array.
// It never proves an index into a zero-length array.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 32 and 36
//   - rules/errors/panic.md — § 15.6 "Assertion refinement"
func (a *Analyzer) refinementProvesArrayIndex(array, index ast.Expression, indexType Type, length *big.Int) (ArrayIndexProofKind, bool) {
	if array == nil || index == nil || length == nil || length.Sign() == 0 {
		return "", false
	}
	lower := integerTypeMinimumIsNonNegative(indexType)
	upper := false
	proofKinds := map[ArrayIndexProofKind]bool{}
	for _, refinement := range a.activeConditionFacts {
		if refinement.epoch != a.arrayIndexMutationEpoch {
			continue
		}
		conditionLower, conditionUpper := a.conditionBoundsArrayIndex(refinement.fact.Condition, array, index, length)
		proof := ArrayIndexProofOther
		switch refinement.fact.Kind {
		case ConditionFactBranchTrue, ConditionFactLogicalRHSTrue:
			proof = ArrayIndexProofBranch
		case ConditionFactAssertionSuccess:
			proof = ArrayIndexProofAssertion
		case ConditionFactLogicalRHSFalse:
			continue
		}
		if conditionLower && !lower {
			lower = true
			proofKinds[proof] = true
		}
		if conditionUpper && !upper {
			upper = true
			proofKinds[proof] = true
		}
	}
	if !lower || !upper || len(proofKinds) == 0 {
		return "", false
	}
	if len(proofKinds) == 1 {
		for proof := range proofKinds {
			return proof, true
		}
	}
	return ArrayIndexProofOther, true
}

// conditionBoundsArrayIndex recognizes only conjunctions of exact integer
// comparisons. Disjunctions and unrelated expressions cannot establish facts.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — section 36
func (a *Analyzer) conditionBoundsArrayIndex(condition, array, index ast.Expression, length *big.Int) (bool, bool) {
	comparison, ok := condition.(*ast.InfixExpression)
	if !ok || comparison == nil {
		return false, false
	}
	if comparison.Operator == "&&" {
		leftLower, leftUpper := a.conditionBoundsArrayIndex(comparison.Left, array, index, length)
		rightLower, rightUpper := a.conditionBoundsArrayIndex(comparison.Right, array, index, length)
		return leftLower || rightLower, leftUpper || rightUpper
	}
	leftIndex := a.sameResolvedExpression(comparison.Left, index)
	rightIndex := a.sameResolvedExpression(comparison.Right, index)
	if !leftIndex && !rightIndex {
		return false, false
	}
	if comparison.Operator == "==" {
		other := comparison.Right
		if rightIndex {
			other = comparison.Left
		}
		constant, constantOK := a.integerConstantValue(other)
		return constantOK && constant.Sign() >= 0, constantOK && constant.Sign() >= 0 && constant.Cmp(length) < 0
	}
	if leftIndex {
		switch comparison.Operator {
		case ">=":
			return a.lowerComparisonProvesNonNegative(comparison.Right, false), false
		case ">":
			return a.lowerComparisonProvesNonNegative(comparison.Right, true), false
		case "<":
			return false, a.upperComparisonProvesInBounds(comparison.Right, array, length, false)
		case "<=":
			return false, a.upperComparisonProvesInBounds(comparison.Right, array, length, true)
		}
	}
	switch comparison.Operator {
	case "<=":
		return a.lowerComparisonProvesNonNegative(comparison.Left, false), false
	case "<":
		return a.lowerComparisonProvesNonNegative(comparison.Left, true), false
	case ">":
		return false, a.upperComparisonProvesInBounds(comparison.Left, array, length, false)
	case ">=":
		return false, a.upperComparisonProvesInBounds(comparison.Left, array, length, true)
	}
	return false, false
}

// lowerComparisonProvesNonNegative evaluates the exact constant side of a
// lower-bound comparison without host-width narrowing.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 35-36
func (a *Analyzer) lowerComparisonProvesNonNegative(bound ast.Expression, strict bool) bool {
	constant, ok := a.integerConstantValue(bound)
	if !ok {
		return false
	}
	if strict {
		return constant.Cmp(big.NewInt(-1)) >= 0
	}
	return constant.Sign() >= 0
}

// upperComparisonProvesInBounds accepts an exact constant boundary or the Len
// property of the same resolved fixed-array binding.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 35-36
func (a *Analyzer) upperComparisonProvesInBounds(bound, array ast.Expression, length *big.Int, inclusive bool) bool {
	if !inclusive && a.isLengthOfArray(bound, array) {
		return true
	}
	constant, ok := a.integerConstantValue(bound)
	if !ok {
		return false
	}
	if inclusive {
		return constant.Cmp(length) < 0
	}
	return constant.Cmp(length) <= 0
}

// sameResolvedExpression compares identifier/member paths by compiler-owned
// binding identity so shadowed names cannot forge a bounds proof.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — section 36
func (a *Analyzer) sameResolvedExpression(left, right ast.Expression) bool {
	switch left := left.(type) {
	case *ast.Identifier:
		right, ok := right.(*ast.Identifier)
		if !ok {
			return false
		}
		leftBinding, leftOK := a.ResolvedBindingOf(left)
		rightBinding, rightOK := a.ResolvedBindingOf(right)
		return leftOK && rightOK && leftBinding.ID == rightBinding.ID
	case *ast.MemberExpression:
		right, ok := right.(*ast.MemberExpression)
		return ok && left.Property != nil && right.Property != nil &&
			left.Property.Value == right.Property.Value && a.sameResolvedExpression(left.Object, right.Object)
	default:
		return false
	}
}

// isLengthOfArray recognizes only the canonical/accepted Len property on the
// exact resolved array expression used by the indexed access.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — section 36
//   - rules/library/core-library.md — fixed-array Len
func (a *Analyzer) isLengthOfArray(expression, array ast.Expression) bool {
	member, ok := expression.(*ast.MemberExpression)
	if !ok || member == nil || member.Property == nil || member.Object == nil {
		return false
	}
	return (member.Property.Value == "Len" || member.Property.Value == "len") && a.sameResolvedExpression(member.Object, array)
}

// integerTypeMinimumIsNonNegative derives only the lower-bound half of the
// Package 14 index predicate from canonical integer range facts.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — section 36
//   - rules/types/contracts.md — range contracts
func integerTypeMinimumIsNonNegative(indexType Type) bool {
	if indexType.MinInteger == nil {
		return false
	}
	minimum := new(big.Int).Set(indexType.MinInteger)
	for _, contract := range indexType.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if ok && rangeContract.Min != nil && rangeContract.Min.Cmp(minimum) > 0 {
			minimum.Set(rangeContract.Min)
		}
	}
	return minimum.Sign() >= 0
}

// indexErrorType resolves the core declaration when it is part of the module
// graph and otherwise provides the same compiler-owned semantic identity for
// standalone Sema analysis. IndexError remains declarable by sec/core and is
// not reserved as a language intrinsic.
// Rules: rules/mlir/packages/sec-mlir-dialect_package14.md — §§48–53;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §53.
func (a *Analyzer) indexErrorType() Type {
	if typ, ok := a.types["IndexError"]; ok && typ.Kind != InvalidType {
		return typ
	}
	return Type{
		Name:            "IndexError",
		Kind:            EnumType,
		Underlying:      "uint",
		ErrorAssignable: true,
		EnumValues:      []string{"OutOfBounds"},
		EnumConsts:      builtinEnumConsts([]string{"OutOfBounds"}),
	}
}

// integerTypeRangeArrayIndexProof distinguishes a named integer range from an
// inline declaration contract while proving the same exact 0 <= index < N
// predicate.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 32 and 36
//   - rules/types/contracts.md — range contracts
func integerTypeRangeArrayIndexProof(indexType Type, length *big.Int) (ArrayIndexProofKind, bool) {
	if length == nil || length.Sign() == 0 || indexType.MinInteger == nil || indexType.MaxInteger == nil {
		return "", false
	}
	minimum := new(big.Int).Set(indexType.MinInteger)
	maximum := new(big.Int).Set(indexType.MaxInteger)
	hasRangeContract := false
	for _, contract := range indexType.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if !ok {
			continue
		}
		hasRangeContract = true
		if rangeContract.Min != nil && rangeContract.Min.Cmp(minimum) > 0 {
			minimum.Set(rangeContract.Min)
		}
		if rangeContract.Max != nil {
			contractMaximum := new(big.Int).Set(rangeContract.Max)
			if rangeContract.Exclusive {
				contractMaximum.Sub(contractMaximum, big.NewInt(1))
			}
			if contractMaximum.Cmp(maximum) < 0 {
				maximum.Set(contractMaximum)
			}
		}
	}
	if minimum.Sign() < 0 || maximum.Cmp(length) >= 0 {
		return "", false
	}
	if hasRangeContract && !indexType.Named {
		return ArrayIndexProofContract, true
	}
	return ArrayIndexProofRange, true
}

// arrayIndexTransferAction selects existing collection element transfer actions.
// Rules: rules/collections/collections.md — §8; rules/mlir/packages/sec-mlir-dialect_package14.md — §§31–36.
func arrayIndexTransferAction(elementType Type, use ArrayIndexUseKind) ResolvedArrayTransferAction {
	switch use {
	case ArrayIndexWrite:
		return ArrayTransferConstructDirect
	case ArrayIndexBorrow:
		return ArrayTransferBorrowShared
	case ArrayIndexMutBorrow:
		return ArrayTransferBorrowMutable
	}
	switch CopyClassificationOf(elementType) {
	case CopyTrivial:
		return ArrayTransferCopyTrivial
	case CopySemantic:
		return ArrayTransferCopySemantic
	default:
		return ArrayTransferMove
	}
}

// setResolvedIndexUse updates access mode and transfer action on exactly the
// collection-specific fact already established during expression inference.
//
// Rules:
//   - rules/collections/collections.md — §8.1 and §8.4
func (a *Analyzer) setResolvedIndexUse(expr *ast.IndexExpression, use ArrayIndexUseKind) {
	plan, ok := a.resolvedArrayIndexPlans[expr]
	if ok {
		plan.UseKind = use
		plan.Action = arrayIndexTransferAction(plan.ElementType, use)
		a.recordResolvedArrayIndexPlan(expr, plan)
	}
	if slice, ok := a.resolvedSliceIndexPlans[expr]; ok {
		slice.UseKind = use
		slice.Action = arrayIndexTransferAction(slice.ElementType, use)
		a.resolvedSliceIndexPlans[expr] = cloneSliceIndexPlan(slice)
	}
	listPlan, listOK := a.resolvedListIndexPlans[expr]
	if listOK {
		listPlan.UseKind = use
		listPlan.Action = arrayIndexTransferAction(listPlan.ElementType, use)
		a.recordResolvedListIndexPlan(expr, listPlan)
	}
}

// sequenceIndexProof shares the exact constant, integer-domain and dominating
// refinement proof engine between fixed arrays and known-length borrowed slices.
// Unknown length is not positive evidence, even for a constant index.
// Rules: rules/mlir/packages/sec-mlir-dialect_package14.md — §§32,35–36;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§51–52.
func (a *Analyzer) sequenceIndexProof(expr *ast.IndexExpression, indexType Type, length *big.Int) (*big.Int, ArrayIndexProofKind) {
	constant, known := a.integerConstantValue(expr.Index)
	constant = clonePlaceConstantIndex(constant)
	if length == nil || length.Sign() <= 0 {
		return constant, ""
	}
	if known {
		if constant.Sign() >= 0 && constant.Cmp(length) < 0 {
			return constant, ArrayIndexProofConstant
		}
		return constant, ""
	}
	if proof, ok := integerTypeRangeArrayIndexProof(indexType, length); ok {
		return nil, proof
	}
	if proof, ok := a.refinementProvesArrayIndex(expr.Left, expr.Index, indexType, length); ok {
		return nil, proof
	}
	return nil, ""
}
