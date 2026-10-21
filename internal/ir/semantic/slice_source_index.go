package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// lowerSliceElementPlace evaluates the source and original index once, then
// validates temporal and spatial access in that order. Only the successful
// edge forms a Place; no element or holder is loaded or transferred.
// Rules: rules/compiler/semantic_ir.md — §126.15–20;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§45–46,50–58,81–82.
func (fb *functionBuilder) lowerSliceElementPlace(expr *ast.IndexExpression, plan sema.ResolvedSliceIndexPlan) (builtValue, error) {
	source, facts, err := fb.sliceSourceValue(expr.Left)
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
		return builtValue{}, fmt.Errorf("slice index operand differs from its resolved integer type")
	}
	loc := location(expr.Token)
	validity, err := fb.validateReference(source, facts, loc)
	if err != nil {
		return builtValue{}, err
	}
	element, err := fb.owner.internType(plan.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	sourceType, _ := fb.owner.module.Types.Lookup(source.typ)
	if sourceType.Element != element || (sourceType.Kind == TypeMutableSlice) != plan.Mutable {
		return builtValue{}, fmt.Errorf("slice index plan differs from its source element or authority")
	}
	op := Operation{Kind: OpSliceElementPlace, Operands: []ValueID{source.id, index.id}, ArrayIndexSigned: plan.IndexSigned, Reference: cloneReferenceMetadata(facts), ReferenceGuard: validity, Location: loc}
	if plan.CheckKind == sema.ArrayIndexProvenSafe {
		proof, ok := semanticArrayIndexProof(plan.ProofKind)
		if !ok || plan.FailureMode != sema.ArrayIndexFailureNone {
			return builtValue{}, fmt.Errorf("slice index has inconsistent proven-safe facts")
		}
		op.ArrayCheckKind, op.ArrayProofKind = ArrayIndexProvenSafe, proof
		op.Integer = cloneBigInt(plan.ConstantIndex)
	} else {
		if plan.CheckKind != sema.ArrayIndexRuntimeCheck {
			return builtValue{}, fmt.Errorf("slice index has unknown check kind %q", plan.CheckKind)
		}
		_, protected := fb.failurePoint(expr)
		if protected != (plan.FailureMode == sema.ArrayIndexFailureFallible) || !protected && plan.FailureMode != sema.ArrayIndexFailureOrdinary {
			return builtValue{}, fb.unsupported("slice index failure mode without its resolved try boundary", expr.Token)
		}
		boolean := fb.owner.module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
		predicate := fb.result(Operation{Kind: OpSliceIndexInBounds, Operands: op.Operands, ArrayIndexSigned: plan.IndexSigned, ReferenceGuard: validity, Location: loc}, boolean)
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
			fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: "slice-index", Location: loc})
		}
		fb.current = success
		op.ArrayCheckKind, op.ArrayProofKind, op.ArrayGuard = ArrayIndexRuntimeCheck, ArrayIndexProofGuarded, predicate.id
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: element, Mutable: plan.Mutable})
	return fb.placeResult(op, placeType), nil
}

// lowerSliceIndexRead consumes the resolved trivial-read action and reads the
// guarded element Place using P15, retaining the non-owning source holder.
// Rules: rules/compiler/semantic_ir.md — §126.19–20;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–55,58.
func (fb *functionBuilder) lowerSliceIndexRead(expr *ast.IndexExpression, plan sema.ResolvedSliceIndexPlan) (builtValue, error) {
	if plan.UseKind != sema.ArrayIndexRead || plan.Action != sema.ArrayTransferCopyTrivial {
		return builtValue{}, fb.unsupported("non-trivial slice element transfer or non-read use (P17)", expr.Token)
	}
	place, err := fb.lowerSliceElementPlace(expr, plan)
	if err != nil {
		return builtValue{}, err
	}
	typ, _ := fb.owner.module.Types.Lookup(place.typ)
	if !package14TrivialValue(fb.owner.module, typ.Element) {
		return builtValue{}, fb.unsupported("slice element read outside the P15 trivial value surface (P17/P16-36)", expr.Token)
	}
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: location(expr.Token)}, typ.Element), nil
}

// lowerSliceIndexWrite establishes the destination Place before evaluating
// the RHS and commits a trivial replacement through P15 PlaceWrite.
// Rules: rules/foundations/operators.md — "Assignment evaluation order";
// rules/compiler/semantic_ir.md — §126.19–20;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58.
func (fb *functionBuilder) lowerSliceIndexWrite(stmt *ast.AssignmentStatement, index *ast.IndexExpression, plan sema.ResolvedSliceIndexPlan) error {
	if stmt.Operator != "=" || plan.UseKind != sema.ArrayIndexWrite || plan.Action != sema.ArrayTransferConstructDirect || !plan.Mutable || !sema.TriviallyDestructible(plan.ElementType) || sema.CopyClassificationOf(plan.ElementType) != sema.CopyTrivial {
		return fb.unsupported("compound or non-trivial slice element replacement (P17)", stmt.Token)
	}
	place, err := fb.lowerSliceElementPlace(index, plan)
	if err != nil {
		return err
	}
	typ, _ := fb.owner.module.Types.Lookup(place.typ)
	if !package14TrivialValue(fb.owner.module, typ.Element) {
		return fb.unsupported("slice element replacement outside the P15 trivial value surface (P17/P16-36)", stmt.Token)
	}
	value, err := fb.buildExpr(stmt.Value, typ.Element)
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: location(stmt.Token)})
	return nil
}

// lowerSliceElementBorrow applies the resolved P15 borrow facts to a bounded
// element Place, retaining the parent borrow without increasing authority.
// Rules: rules/compiler/semantic_ir.md — §126.19;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,60–63.
func (fb *functionBuilder) lowerSliceElementBorrow(expr *ast.RefExpression, index *ast.IndexExpression, plan sema.ResolvedSliceIndexPlan) (builtValue, error) {
	reference, ok := fb.owner.analyzer.ResolvedReferencePlanOf(expr)
	if !ok || !reference.Facts.Checkable() || reference.Facts.MutableAuthority != expr.Mutable {
		return builtValue{}, fb.unsupported("slice element borrow without complete resolved reference facts", expr.Token)
	}
	use, action := sema.ArrayIndexBorrow, sema.ArrayTransferBorrowShared
	if expr.Mutable {
		use, action = sema.ArrayIndexMutBorrow, sema.ArrayTransferBorrowMutable
	}
	if plan.UseKind != use || plan.Action != action || expr.Mutable && !plan.Mutable {
		return builtValue{}, fmt.Errorf("slice element borrow differs from its resolved use/authority")
	}
	place, err := fb.lowerSliceElementPlace(index, plan)
	if err != nil {
		return builtValue{}, err
	}
	return fb.emitBorrow(expr, reference, place)
}
