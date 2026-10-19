package sema

import (
	"math/big"

	"sec/internal/ast"
)

// P16 creation plans are semantic facts, independent of a physical descriptor.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 33–37.
type ResolvedSliceSourceKind string
type ResolvedSliceMode string
type SliceRangeCheckKind string

const (
	SliceSourceFixedArray   ResolvedSliceSourceKind = "fixed-array"
	SliceSourceSlice        ResolvedSliceSourceKind = "slice"
	SliceSourceDynamicArray ResolvedSliceSourceKind = "dynamic-array"
	SliceShared             ResolvedSliceMode       = "shared"
	SliceMutable            ResolvedSliceMode       = "mutable"
	SliceRangeProvenSafe    SliceRangeCheckKind     = "proven-safe"
	SliceRangeRuntimeCheck  SliceRangeCheckKind     = "runtime-check"
)

// StaticSliceRange is a source-relative exact normalized half-open range.
// The source Place and reference origins separately retain ultimate storage.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 14–16, 25, 36.
type StaticSliceRange struct{ Start, EndExclusive *big.Int }

// ResolvedSliceEndpoint retains the original source expression's type and
// exact constant, without inventing an expression for an omitted endpoint.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 15, 35.
type ResolvedSliceEndpoint struct {
	Present       bool
	Type          Type
	Signed        bool
	Constant      *big.Int
	ConstantKnown bool
}

type ResolvedSliceCreationPlan struct {
	EvaluationOrder []SliceOperand
	Mode            ResolvedSliceMode
	SourceKind      ResolvedSliceSourceKind
	ElementType     Type
	SourcePlace     Place
	SourceSliceType Type
	SourceLength    *big.Int
	Start, End      ResolvedSliceEndpoint
	EndExclusive    bool
	CheckKind       SliceRangeCheckKind
	ProofKind       string
	StaticRange     *StaticSliceRange
	DynamicRange    SliceRangeID
	BorrowID        BorrowID
	LifetimeID      LifetimeID
	ReferenceFacts  ResolvedReferencePlan
	ErrorType       Type
}

// resolvedSliceEndpoint records an already typed bound during analysis, never
// during a public plan query. Exact constants are copied from the CTE result.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 35–37.
func (a *Analyzer) resolvedSliceEndpoint(expr ast.Expression) ResolvedSliceEndpoint {
	if expr == nil {
		return ResolvedSliceEndpoint{}
	}
	typ := a.expressionTypes[expr]
	value, known := a.integerConstantValue(expr)
	return ResolvedSliceEndpoint{Present: true, Type: semanticSnapshotType(typ), Signed: typ.Kind == IntType,
		Constant: clonePlaceConstantIndex(value), ConstantKnown: known}
}

// sliceSourcePlace retains the original source's canonical storage origin
// before applying the new range; existing reference/slice normalization owns
// this derivation, rather than a second slice-specific Place model.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 32, 36, 59–62.
func (a *Analyzer) sliceSourcePlace(expr ast.Expression, typ Type) (Place, bool) {
	place, ok := a.resolvePlace(expr)
	if !ok && typ.Kind == ReferenceType && typ.Element != nil {
		// A returned reference is a value with existing P15 origin facts, not
		// a fresh temporary storage root. Reuse those facts without re-calling
		// or re-inferring its source expression.
		// Rules: rules/memory/borrowing.md §§12,15;
		// rules/mlir/packages/sec-mlir-dialect_package16.md §§17,32,39–43.
		place, ok = a.referencePlaceOrigin(expr)
		if ok {
			place.Type = typ
		}
	}
	if !ok {
		return Place{}, false
	}
	if typ.Kind == ReferenceType && typ.Element != nil {
		place = a.canonicalDereferencePlace(expr, place, typ)
	} else if typ.Kind == SliceType {
		place = a.canonicalSlicePlace(expr, place)
	}
	return place, true
}

// recordSliceCreationPlan publishes source, endpoint, range and existing P15
// borrow facts once inference has accepted an explicit ref-of-range expression.
// Known bounds use the canonical classifier before inclusive normalization;
// an unknown length or bound never becomes a safety proof.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 14–19, 33–38.
func (a *Analyzer) recordSliceCreationPlan(ref *ast.RefExpression) {
	if a.summaryPass {
		return
	}
	rangeExpr, ok := ref.Value.(*ast.SliceExpression)
	if !ok {
		return
	}
	delete(a.resolvedSliceCreationPlans, ref)
	reference, ok := a.resolvedReferencePlans[ref]
	if !ok {
		return
	}
	sourceType := a.expressionTypes[rangeExpr.Left]
	container := dereferenceType(sourceType)
	if container.Element == nil {
		return
	}
	source, ok := a.sliceSourcePlace(rangeExpr.Left, sourceType)
	if !ok {
		return
	}
	plan := ResolvedSliceCreationPlan{EvaluationOrder: sliceEvaluationOrder(rangeExpr.Start != nil, rangeExpr.End != nil),
		Mode: SliceShared, ElementType: *container.Element,
		SourcePlace: source, SourceSliceType: sourceType, Start: a.resolvedSliceEndpoint(rangeExpr.Start),
		End: a.resolvedSliceEndpoint(rangeExpr.End), EndExclusive: rangeExpr.Exclusive,
		CheckKind: SliceRangeRuntimeCheck, ReferenceFacts: reference,
		BorrowID: reference.Facts.Borrow, LifetimeID: reference.Facts.Lifetime,
		ErrorType: a.types["RangeError"]}
	if ref.Mutable {
		plan.Mode = SliceMutable
	}
	switch container.Kind {
	case SliceType:
		plan.SourceKind = SliceSourceSlice
		plan.SourceLength = staticSliceExtent(source)
	case ArrayType:
		if length, fixed := exactFixedArrayLength(container); fixed {
			plan.SourceKind, plan.SourceLength = SliceSourceFixedArray, length
		} else {
			plan.SourceKind = SliceSourceDynamicArray
		}
	default:
		return
	}
	facts := SliceRangeEndpoints{Start: plan.Start.Constant, End: plan.End.Constant,
		StartOmitted: !plan.Start.Present, EndOmitted: !plan.End.Present,
		Inclusive: !plan.EndExclusive, Length: plan.SourceLength}
	start, end, classification := normalizeSliceRange(facts)
	if classification.Failure != SliceRangeValid {
		return
	}
	if start != nil && end != nil {
		plan.StaticRange = &StaticSliceRange{start, end}
		if plan.SourceLength != nil {
			plan.CheckKind, plan.ProofKind = SliceRangeProvenSafe, "constant"
			if !plan.Start.Present && !plan.End.Present {
				plan.ProofKind = "source-open-bound"
			}
		}
	}
	if raw := a.resolvedPlaces[rangeExpr]; len(raw.Projections) > 0 {
		plan.DynamicRange = raw.Projections[len(raw.Projections)-1].DynamicRange
	}
	// The complete open range is valid for every extent, including zero;
	// this proof does not invent a numeric source length or static range.
	// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§16,78,119.
	if !plan.Start.Present && !plan.End.Present {
		plan.CheckKind, plan.ProofKind = SliceRangeProvenSafe, "source-open-bound"
	}
	a.resolvedSliceCreationPlans[ref] = cloneSliceCreationPlan(plan)
	a.recordSliceRangeEffect(ref, plan)
}

// ResolvedSliceCreationPlanOf reads a recorded plan without evaluating
// endpoints, resolving Places, proving bounds or registering a new borrow.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — § 37.
func (a *Analyzer) ResolvedSliceCreationPlanOf(ref *ast.RefExpression) (ResolvedSliceCreationPlan, bool) {
	if a == nil || ref == nil {
		return ResolvedSliceCreationPlan{}, false
	}
	plan, ok := a.resolvedSliceCreationPlans[ref]
	if !ok {
		return ResolvedSliceCreationPlan{}, false
	}
	return cloneSliceCreationPlan(plan), true
}

// inferSliceExpression checks the source and endpoint types in source order
// before range validation, and retains the existing reference origins.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 13–19, 35–38;
// rules/collections/collections.md — slice borrowing.
func (a *Analyzer) inferSliceExpression(expr *ast.SliceExpression) (Type, expressionValue) {
	leftType, _ := a.inferPlaceBase(expr.Left)
	if leftType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	leftType = dereferenceType(leftType)
	if leftType.Kind != ArrayType && leftType.Kind != SliceType {
		a.addErrorAtToken(expr.Token, "type %s cannot be sliced", typeDisplayName(leftType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if leftType.Element == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	// Resolve the original endpoint types in source order. The slice plan
	// retains these facts rather than re-inferring them from a later query.
	// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 17, 35–37.
	for _, operand := range sliceEvaluationOrder(expr.Start != nil, expr.End != nil)[1:] {
		endpoint := expr.Start
		if operand == SliceOperandEnd {
			endpoint = expr.End
		}
		if endpoint != nil {
			endpointType, _ := a.inferExpression(endpoint)
			if endpointType.Kind == InvalidType {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			if !isIntegerType(endpointType) {
				a.addErrorAtToken(expressionToken(endpoint), "slice bound must be an integer, got %s", typeDisplayName(endpointType))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
		}
	}
	a.checkSliceBounds(expr, leftType)
	originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(expr.Left)
	return Type{
		Name:                        typeDisplayName(*leftType.Element) + "[]",
		Kind:                        SliceType,
		Element:                     leftType.Element,
		ReferenceOriginName:         originName,
		ReferenceOriginToken:        originToken,
		ReferenceOriginLocal:        originLocal,
		ReferenceOriginStorage:      originStorage,
		ReferenceOriginGeneration:   generation,
		ReferenceOriginMatchScoped:  a.referenceOriginMatchScopedForExpression(expr.Left),
		ReferenceOriginAlternatives: a.referenceOriginAlternativesForExpression(expr.Left),
	}, expressionValue{Display: expr.String()}
}
