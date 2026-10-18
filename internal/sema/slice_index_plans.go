package sema

import (
	"math/big"
	"sec/internal/ast"
)

// ResolvedSliceIndexPlan records source authority, spatial proof and element
// use without choosing an element value representation or moving ownership.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58.
type ResolvedSliceIndexPlan struct {
	SliceType     Type
	ElementType   Type
	Mutable       bool
	SourcePlace   Place
	SliceLength   *big.Int
	IndexType     Type
	IndexSigned   bool
	ConstantIndex *big.Int
	CheckKind     ArrayIndexCheckKind
	ProofKind     ArrayIndexProofKind
	UseKind       ArrayIndexUseKind
	Action        ResolvedArrayTransferAction
	FailureMode   ArrayIndexFailureMode
	ErrorType     Type
}

// sliceIndexLength reads established local range provenance during inference.
// It never guesses the extent of a dynamic or ambiguous source.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§28–30,51–52.
func (a *Analyzer) sliceIndexLength(expr *ast.IndexExpression) *big.Int {
	source, ok := a.sliceSourcePlace(expr.Left, a.expressionTypes[expr.Left])
	if !ok {
		return nil
	}
	return staticSliceExtent(source)
}

// recordSliceIndexPlan extends collection index producers, reusing their
// proof and use/action vocabulary. A spatial proof removes only the bounds
// effect; it makes no promise about P15 reference validity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,118–120.
func (a *Analyzer) recordSliceIndexPlan(expr *ast.IndexExpression, sourceType, sliceType, elementType, indexType Type) {
	if a.summaryPass {
		return
	}
	source, _ := a.sliceSourcePlace(expr.Left, sourceType)
	length := staticSliceExtent(source)
	constant, proof := a.sequenceIndexProof(expr, indexType, length)
	plan := ResolvedSliceIndexPlan{
		SliceType: sliceType, ElementType: elementType,
		Mutable: sourceType.ReferenceMutable, SourcePlace: source, SliceLength: length,
		IndexType: indexType, IndexSigned: indexType.Kind == IntType, ConstantIndex: constant,
		CheckKind: ArrayIndexRuntimeCheck, ProofKind: proof, UseKind: ArrayIndexRead,
		Action:      arrayIndexTransferAction(elementType, ArrayIndexRead),
		FailureMode: ArrayIndexFailureOrdinary, ErrorType: a.indexErrorType(),
	}
	if proof != "" {
		plan.CheckKind, plan.FailureMode = ArrayIndexProvenSafe, ArrayIndexFailureNone
	}
	if a.resolvedSliceIndexPlans == nil {
		a.resolvedSliceIndexPlans = map[*ast.IndexExpression]ResolvedSliceIndexPlan{}
	}
	a.resolvedSliceIndexPlans[expr] = cloneSliceIndexPlan(plan)
	a.recordArrayIndexEffect(expr, ResolvedArrayIndexPlan{CheckKind: plan.CheckKind, FailureMode: plan.FailureMode})
}

// cloneSliceIndexPlan detaches every mutable source, origin, type and exact
// integer, including canonical error variant data.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,51.
func cloneSliceIndexPlan(plan ResolvedSliceIndexPlan) ResolvedSliceIndexPlan {
	plan.SliceType = semanticSnapshotType(plan.SliceType)
	plan.ElementType = semanticSnapshotType(plan.ElementType)
	plan.IndexType = semanticSnapshotType(plan.IndexType)
	plan.SourcePlace = cloneSlicePlanPlace(plan.SourcePlace)
	plan.SliceLength = clonePlaceConstantIndex(plan.SliceLength)
	plan.ConstantIndex = clonePlaceConstantIndex(plan.ConstantIndex)
	// Creation and index plans share the exact same canonical error snapshot.
	plan.ErrorType = cloneSlicePlanErrorType(plan.ErrorType)
	return plan
}

// ResolvedSliceIndexPlanOf is a read-only query over facts established during
// inference; it does not evaluate an index, prove a bound or create a borrow.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,51.
func (a *Analyzer) ResolvedSliceIndexPlanOf(expr *ast.IndexExpression) (ResolvedSliceIndexPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedSliceIndexPlan{}, false
	}
	plan, ok := a.resolvedSliceIndexPlans[expr]
	if !ok {
		return ResolvedSliceIndexPlan{}, false
	}
	return cloneSliceIndexPlan(plan), true
}
