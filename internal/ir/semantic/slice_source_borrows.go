package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// lowerSliceCreation consumes the captured explicit-borrow plan. The source
// Place/slice value precedes each present endpoint in the recorded order;
// dynamic slice validity precedes length/range observation. No backing array
// value is loaded and no source slice holder is transferred.
// Rules: rules/compiler/semantic_ir.md — §126.6–8,11–13;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,20–23,33–37,42–46,78–80.
func (fb *functionBuilder) lowerSliceCreation(expr *ast.RefExpression, plan sema.ResolvedSliceCreationPlan) (builtValue, error) {
	rangeExpr := expr.Value.(*ast.SliceExpression)
	mutable := plan.Mode == sema.SliceMutable
	if plan.Mode != sema.SliceShared && plan.Mode != sema.SliceMutable || mutable != plan.ReferenceFacts.Facts.MutableAuthority {
		return builtValue{}, fmt.Errorf("slice creation plan has inconsistent resolved authority")
	}
	reborrow := plan.SourceKind == sema.SliceSourceSlice
	if !plan.HasValidEvaluationOrder() || !reborrow && plan.SourceLength == nil {
		return builtValue{}, fmt.Errorf("slice plan lacks source extent or operand schedule")
	}
	var place builtValue
	var parent *ReferenceMetadata
	var err error
	if reborrow {
		place, parent, err = fb.sliceSourceValue(rangeExpr.Left)
	} else {
		sourcePlan := plan.ReferenceFacts
		sourcePlan.Place = plan.SourcePlace
		sourceExpr := &ast.RefExpression{Token: expr.Token, Mutable: mutable, Value: rangeExpr.Left}
		place, parent, err = fb.fixedSliceSourcePlace(sourceExpr, sourcePlan)
	}
	if err != nil {
		return builtValue{}, err
	}
	loc := location(expr.Token)
	uintType := fb.owner.module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	operands := []ValueID{0}
	if !reborrow {
		extent := fb.result(Operation{Kind: OpConstInt, Integer: cloneBigInt(plan.SourceLength), Location: loc}, uintType)
		operands[0] = extent.id
	}
	metadata := &SliceRangeMetadata{StartPresent: plan.Start.Present, EndPresent: plan.End.Present, EndExclusive: plan.EndExclusive}
	for _, operand := range plan.EvaluationOrder[1:] {
		endpoint, source := plan.Start, rangeExpr.Start
		if operand == sema.SliceOperandEnd {
			endpoint, source = plan.End, rangeExpr.End
		}
		typ, err := fb.owner.internType(endpoint.Type)
		if err != nil {
			return builtValue{}, err
		}
		value, err := fb.buildExpr(source, typ)
		if err != nil {
			return builtValue{}, err
		}
		operands = append(operands, value.id)
		if operand == sema.SliceOperandStart {
			metadata.StartType, metadata.StartSigned = value.typ, endpoint.Signed
		} else {
			metadata.EndType, metadata.EndSigned = value.typ, endpoint.Signed
		}
	}
	guard := ValueID(0)
	if reborrow {
		guard, err = fb.validateReference(place, parent, loc)
		if err != nil {
			return builtValue{}, err
		}
		extent := fb.result(Operation{Kind: OpSliceLength, Operands: []ValueID{place.id}, ReferenceGuard: guard, Location: loc}, uintType)
		operands[0] = extent.id
	}
	start, length := fb.newValue(uintType, OwnershipImmediate, loc), fb.newValue(uintType, OwnershipImmediate, loc)
	normalize := Operation{Kind: OpSliceRangeNormalize, Operands: operands, Results: []Value{start, length}, SliceRange: metadata, Location: loc}
	switch plan.CheckKind {
	case sema.SliceRangeProvenSafe:
		proof, valid := semanticArrayIndexProof(sema.ArrayIndexProofKind(plan.ProofKind))
		if plan.ProofKind == "source-open-bound" {
			proof, valid = SliceRangeProofOpenBound, true
		}
		if !valid {
			return builtValue{}, fmt.Errorf("unsupported resolved slice range proof %q", plan.ProofKind)
		}
		metadata.ProofKind = proof
		fb.emit(normalize)
	case sema.SliceRangeRuntimeCheck:
		errorType, err := fb.owner.internType(plan.ErrorType)
		if err != nil {
			return builtValue{}, err
		}
		boolean := fb.owner.module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
		failed, reason := fb.newValue(boolean, OwnershipImmediate, loc), fb.newValue(errorType, OwnershipImmediate, loc)
		normalize.Kind = OpSliceRangeCheck
		normalize.Results = append(normalize.Results, failed, reason)
		fb.emit(normalize)
		failure, success := fb.newBlock(), fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: failure.ID}, {Block: success.ID}}, Location: loc})
		fb.current = failure
		if _, protected := fb.failurePoint(expr); protected {
			fb.routeFailure(expr, builtValue{id: reason.ID, typ: errorType}, loc)
		} else {
			fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: "slice-range", Location: loc})
		}
		fb.current = success
	default:
		return builtValue{}, fmt.Errorf("unknown resolved slice range check %q", plan.CheckKind)
	}
	element, err := fb.owner.internType(plan.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	typ, err := fb.owner.module.Types.InternSlice(element, mutable)
	if err != nil {
		return builtValue{}, err
	}
	kind, ownership := OpSliceBorrowShared, OwnershipSharedReference
	if mutable {
		kind, ownership = OpSliceBorrowMutable, OwnershipMutableReference
	}
	if reborrow {
		kind = OpSliceReborrowShared
		if mutable {
			kind = OpSliceReborrowMutable
		}
	}
	value := fb.newValue(typ, ownership, loc)
	facts := referenceMetadataOf(plan.ReferenceFacts.Facts)
	if parent != nil {
		facts.SourceBorrow = parent.Borrow
	}
	fb.emit(Operation{Kind: kind, Operands: []ValueID{place.id, start.ID, length.ID}, Results: []Value{value}, Reference: facts, ReferenceGuard: guard, Location: loc})
	fb.recordReference(value.ID, facts)
	return builtValue{id: value.ID, typ: typ}, nil
}

// fixedSliceSourcePlace reuses P15 Place roots/projections, including a fixed
// array observed through an existing reference. It retains the parent's
// reference facts instead of borrowing a copied aggregate value.
// Rules: rules/compiler/semantic_ir.md — §126.7–8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,42,44.
func (fb *functionBuilder) fixedSliceSourcePlace(expr *ast.RefExpression, plan sema.ResolvedReferencePlan) (builtValue, *ReferenceMetadata, error) {
	if !plan.Reborrow {
		place, err := fb.lowerBorrowPlace(expr, plan)
		return place, nil, err
	}
	holder, steps := referencePath(expr.Value)
	if holder == nil {
		return builtValue{}, nil, fb.unsupported("fixed-array slice source without a supported reference holder", expr.Token)
	}
	reference, facts, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, nil, err
	}
	guard, err := fb.validateReference(reference, facts, location(expr.Token))
	if err != nil {
		return builtValue{}, nil, err
	}
	typ, ok := fb.owner.module.Types.Lookup(reference.typ)
	if !ok || typ.Kind != TypeReference {
		return builtValue{}, nil, fmt.Errorf("fixed-array source holder is not a direct reference")
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: typ.Element, Mutable: typ.Mutable})
	place := fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: facts, ReferenceGuard: guard, Location: location(expr.Token)}, placeType)
	place, err = fb.projectPlace(steps, plan.Place.Projections, place, typ.Mutable, placeMetadataOf(plan.Place))
	return place, facts, err
}

// sliceSourceValue evaluates a slice source once without copying/moving its
// holder. Named holders and freshly constructed expressions keep their P15
// reference facts; signatures, returned values and projected holders retain
// their separately tracked P16-36 gates.
// Rules: rules/compiler/semantic_ir.md — §126.8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,43–46.
func (fb *functionBuilder) sliceSourceValue(expr ast.Expression) (builtValue, *ReferenceMetadata, error) {
	if holder, ok := expr.(*ast.Identifier); ok {
		value, facts, err := fb.referenceHolder(holder)
		if err != nil {
			return builtValue{}, nil, err
		}
		typ, _ := fb.owner.module.Types.Lookup(value.typ)
		if typ.Kind != TypeSharedSlice && typ.Kind != TypeMutableSlice {
			return builtValue{}, nil, fmt.Errorf("slice source holder is not a bounded reference")
		}
		return value, facts, nil
	}
	if ref, ok := expr.(*ast.RefExpression); ok {
		value, handled, err := fb.p16SliceExpression(ref)
		if err != nil {
			return builtValue{}, nil, err
		}
		if handled && fb.references[value.id] != nil {
			return value, fb.references[value.id], nil
		}
	}
	return builtValue{}, nil, fb.unsupported("slice source value or projected holder (P16-36)", expressionToken(expr))
}
