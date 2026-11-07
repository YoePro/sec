package sema

import (
	"math/big"

	"sec/internal/ast"
)

// DynamicArrayIndexPanicOperation is the panic operation of an unproven
// owning-array index (package18 § 42).
const DynamicArrayIndexPanicOperation = "dynamic-array-index"

// ResolvedDynamicArrayIndexPlan is the read-only Sema fact of one index into
// an owning `T[]` (P18-09, P18-10). It reuses the fixed-array and slice index
// vocabulary (package18 § 41: no synonymous enums). The length is runtime
// state, so an access keeps its runtime bounds check: ordinary failure is
// `panic.bounds` with operation dynamic-array-index, and a valid `try`
// selects IndexError.OutOfBounds. The element access never changes the
// backing domain; a borrow of the element depends on the current backing
// epoch (§§ 26, 44–45). Moving a move-only element out by runtime index stays
// rejected (§ 43), so a move action is never consumed.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 40–45
//   - rules/collections/collections.md — § 8 "Indexing"
type ResolvedDynamicArrayIndexPlan struct {
	ArrayType     Type
	ElementType   Type
	IndexType     Type
	IndexSigned   bool
	ConstantIndex *big.Int
	CheckKind     ArrayIndexCheckKind
	ProofKind     ArrayIndexProofKind
	UseKind       ArrayIndexUseKind
	Action        ResolvedArrayTransferAction
	FailureMode   ArrayIndexFailureMode
	ErrorType     Type
	// PanicOperation names the ordinary failure endpoint.
	PanicOperation string
	// Owner is the indexed owner and its backing domain when the indexed
	// value is a published Place.
	Owner             *DynamicArrayOwner
	BackingTransition BackingTransitionKind
	// EpochDependent is set for shared and mutable element borrows.
	EpochDependent bool
}

// recordDynamicArrayIndexPlan publishes the index plan of an owning-array
// element access together with its bounds effect.
func (a *Analyzer) recordDynamicArrayIndexPlan(expr *ast.IndexExpression, arrayType, elementType, indexType Type) {
	a.recordSequenceIndexEffect(expr)
	if a.summaryPass {
		return
	}
	plan := ResolvedDynamicArrayIndexPlan{
		ArrayType: arrayType, ElementType: elementType,
		IndexType: indexType, IndexSigned: indexType.Kind == IntType,
		CheckKind: ArrayIndexRuntimeCheck, UseKind: ArrayIndexRead,
		Action:      arrayIndexTransferAction(elementType, ArrayIndexRead),
		FailureMode: ArrayIndexFailureOrdinary, ErrorType: a.indexErrorType(),
		PanicOperation:    DynamicArrayIndexPanicOperation,
		BackingTransition: BackingTransitionNone,
	}
	if constant, ok := a.integerConstantValue(expr.Index); ok {
		plan.ConstantIndex = new(big.Int).Set(constant)
	}
	if owner, held := a.dynamicArrayOwners[expr.Left]; held {
		owner.Place = clonePlace(owner.Place)
		plan.Owner = &owner
	}
	a.storeDynamicArrayIndexPlan(expr, plan)
}

func (a *Analyzer) storeDynamicArrayIndexPlan(expr *ast.IndexExpression, plan ResolvedDynamicArrayIndexPlan) {
	if a.resolvedDynamicArrayIndexPlans == nil {
		a.resolvedDynamicArrayIndexPlans = map[*ast.IndexExpression]ResolvedDynamicArrayIndexPlan{}
	}
	a.resolvedDynamicArrayIndexPlans[expr] = cloneDynamicArrayIndexPlan(plan)
}

// setDynamicArrayIndexUse updates the use and transfer action once the
// context (write, shared or mutable borrow) is known.
func (a *Analyzer) setDynamicArrayIndexUse(expr *ast.IndexExpression, use ArrayIndexUseKind) {
	plan, ok := a.resolvedDynamicArrayIndexPlans[expr]
	if !ok {
		return
	}
	plan.UseKind = use
	plan.Action = arrayIndexTransferAction(plan.ElementType, use)
	plan.EpochDependent = use == ArrayIndexBorrow || use == ArrayIndexMutBorrow
	a.storeDynamicArrayIndexPlan(expr, plan)
}

// commitFallibleDynamicArrayIndex marks the access fallible once its try
// construct validated.
func (a *Analyzer) commitFallibleDynamicArrayIndex(expr *ast.IndexExpression, errorType Type) {
	plan, ok := a.resolvedDynamicArrayIndexPlans[expr]
	if !ok {
		return
	}
	plan.FailureMode = ArrayIndexFailureFallible
	plan.ErrorType = errorType
	a.storeDynamicArrayIndexPlan(expr, plan)
}

func cloneDynamicArrayIndexPlan(plan ResolvedDynamicArrayIndexPlan) ResolvedDynamicArrayIndexPlan {
	plan.ArrayType = semanticSnapshotType(plan.ArrayType)
	plan.ElementType = semanticSnapshotType(plan.ElementType)
	plan.IndexType = semanticSnapshotType(plan.IndexType)
	plan.ConstantIndex = clonePlaceConstantIndex(plan.ConstantIndex)
	plan.ErrorType = cloneSlicePlanErrorType(plan.ErrorType)
	if plan.Owner != nil {
		owner := *plan.Owner
		owner.Place = clonePlace(owner.Place)
		plan.Owner = &owner
	}
	return plan
}

// ResolvedDynamicArrayIndexPlanOf reads the plan of an owning-array index,
// detached; it evaluates and infers nothing (package18 § 41).
func (a *Analyzer) ResolvedDynamicArrayIndexPlanOf(expr *ast.IndexExpression) (ResolvedDynamicArrayIndexPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedDynamicArrayIndexPlan{}, false
	}
	plan, ok := a.resolvedDynamicArrayIndexPlans[expr]
	if !ok {
		return ResolvedDynamicArrayIndexPlan{}, false
	}
	return cloneDynamicArrayIndexPlan(plan), true
}
