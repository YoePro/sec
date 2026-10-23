package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// Slice holder transfers (P16-36; package16 §§ 39–41). Copying a shared slice
// holder is slice.copy-shared: the bounded reference value is duplicated
// under the same borrow, storage, range and epoch, and no element is copied.
// A mutable slice is move-only: `let moved :<- source` is slice.move-mut,
// which transfers the holder obligation; Sema makes the source unavailable.
// Returned slices are separate (P16-27).

// lowerSliceHolderTransfer lowers a named slice holder used as a value. It
// reports false for a non-slice identifier.
func (fb *functionBuilder) lowerSliceHolderTransfer(holder *ast.Identifier) (builtValue, bool, error) {
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(holder)
	if !ok {
		return builtValue{}, false, nil
	}
	held, ok := fb.bindings[fact.ID]
	if !ok {
		return builtValue{}, false, nil
	}
	typ, _ := fb.owner.module.Types.Lookup(held.typ)
	if typ.Kind != TypeSharedSlice && typ.Kind != TypeMutableSlice {
		return builtValue{}, false, nil
	}
	plan, planned := fb.owner.analyzer.ResolvedTransferPlanOf(holder)
	source, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, true, err
	}
	if use, used := fb.owner.analyzer.ResolvedReferenceUseOf(holder); planned && plan.Site == sema.TransferSiteReturn || used && use.Kind == sema.ReferenceUseReturn {
		// Returning a holder hands over its own bounded reference value: a
		// shared slice is copyable and a mutable one is moved out by the
		// return (package16 § 66; package15 §§ 60-61). The function's
		// return-origin summary is checked by the verifier.
		return source, true, nil
	}
	loc := location(holder.Token)
	kind, ownership := OpSliceCopyShared, OwnershipSharedReference
	if typ.Kind == TypeMutableSlice {
		if !planned || plan.Action != sema.TransferMove {
			return builtValue{}, true, fb.unsupported("mutable slice holder used as a value without a resolved move", holder.Token)
		}
		kind, ownership = OpSliceMoveMutable, OwnershipMutableReference
	} else if planned && plan.Action != sema.TransferCopyTrivial {
		return builtValue{}, true, fb.unsupported("shared slice holder transfer "+string(plan.Action), holder.Token)
	}
	transferred := cloneReferenceMetadata(metadata)
	transferred.SourceBorrow = transferred.Borrow
	value := fb.newValue(source.typ, ownership, loc)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{source.id}, Results: []Value{value}, Reference: transferred, Location: loc})
	fb.recordReference(value.ID, transferred)
	return builtValue{id: value.ID, typ: source.typ}, true, nil
}

// lowerSliceHolderReborrow lowers `ref source` / `ref mut source` over a slice
// holder (MD-061 explicit reborrow) as a reborrow of the holder's complete
// extent under Sema's new borrow. It reports false for a non-slice holder.
func (fb *functionBuilder) lowerSliceHolderReborrow(expr *ast.RefExpression, holder *ast.Identifier) (builtValue, bool, error) {
	source, resolved := fb.owner.analyzer.ResolvedTypeOf(holder)
	if !resolved || !sourceSliceType(source) {
		return builtValue{}, false, nil
	}
	plan, ok := fb.owner.analyzer.ResolvedReferencePlanOf(expr)
	if !ok || !plan.Reborrow || !plan.Facts.Checkable() {
		return builtValue{}, true, fb.unsupported("slice holder reborrow without a resolved reborrow plan", expr.Token)
	}
	value, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, true, err
	}
	result, err := fb.reborrowWholeSlice(value, metadata, expr.Mutable, plan.Facts, location(expr.Token))
	return result, true, err
}
