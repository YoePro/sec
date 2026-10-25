package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Slice parameters (P16-36). A `ref T[]` / `ref mut T[]` parameter is a
// bounded non-owning reference value of the P16 slice type, identified only
// by element and authority; the callee receives the caller-owned borrow with
// Sema's parameter reference facts and an unknown runtime extent.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41, 64–65;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§1–4.

// parameterType returns the Semantic IR type and ownership of a parameter.
func (b *builder) parameterType(parameter sema.FunctionParameter) (TypeID, OwnershipClass, error) {
	if b.maxPackage >= sliceLoweringPackage && sourceSliceType(parameter.Type) {
		typeID, mutable, err := b.internSourceSliceType(parameter.Type)
		if err != nil {
			return 0, "", locateUnsupportedType(err, parameter.Token)
		}
		if mutable {
			return typeID, OwnershipMutableReference, nil
		}
		return typeID, OwnershipSharedReference, nil
	}
	typeID, err := b.internType(parameter.Type)
	if err != nil {
		return 0, "", locateUnsupportedType(err, parameter.Token)
	}
	ownership, err := ownershipForParameter(parameter, b.maxPackage)
	return typeID, ownership, err
}

// internSourceSliceType interns a source `ref T[]` / `ref mut T[]` (or the
// bare slice type) as the P16 slice type of its element and authority.
func (b *builder) internSourceSliceType(t sema.Type) (TypeID, bool, error) {
	mutable := false
	if t.Kind == sema.ReferenceType && t.Element != nil {
		mutable = t.ReferenceMutable
		t = *t.Element
	}
	if t.Kind != sema.SliceType || t.Element == nil {
		return 0, false, &UnsupportedFeatureError{Feature: "slice type " + t.Name, Package: b.maxPackage}
	}
	element, err := b.internType(*t.Element)
	if err != nil {
		return 0, false, err
	}
	typeID, err := b.module.Types.InternSlice(element, mutable)
	return typeID, mutable, err
}

// lowerSliceArgument produces the argument for a slice parameter of type
// param. A fresh slice expression (`ref data[..]`) is built as usual. An
// existing holder is passed as Sema resolved it (§ 65): a shared slice with
// slice.copy-shared under its own borrow, a mutable slice with an implicit
// full-range slice.reborrow-mut under a new borrow, so the caller keeps its
// holder. A mutable holder passed to a shared parameter instead grants a
// full-range shared reborrow; a mutable descriptor is never copied.
// Non-holder mutable slices also use the resolved shared pass, after evaluating
// the original expression once with its own authority and creation/return facts.
// Rules: rules/memory/borrowing.md — §15.3(2)–(3);
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,64–65.
func (fb *functionBuilder) lowerSliceArgument(arg ast.Expression, param TypeID) (builtValue, error) {
	paramType, _ := fb.owner.module.Types.Lookup(param)
	holder, named := arg.(*ast.Identifier)
	if !named {
		var value builtValue
		var err error
		if call, ok := arg.(*ast.CallExpression); ok {
			// The pass is applied below to the returned value; evaluating
			// the call must retain its own result facts and mutable type.
			value, err = fb.buildCall(call)
		} else {
			value, err = fb.buildExpr(arg, param)
		}
		if err != nil {
			return builtValue{}, err
		}
		if value.typ != param {
			sourceType, _ := fb.owner.module.Types.Lookup(value.typ)
			if paramType.Kind == TypeSharedSlice && sourceType.Kind == TypeMutableSlice && sourceType.Element == paramType.Element {
				use, ok := fb.owner.analyzer.ResolvedReferenceUseOf(arg)
				metadata := fb.references[value.id]
				if !ok || use.Kind != sema.ReferenceUsePass || !use.Facts.Checkable() || use.Facts.MutableAuthority || metadata == nil {
					return builtValue{}, fmt.Errorf("mutable slice expression argument has no resolved shared pass")
				}
				return fb.reborrowWholeSlice(value, metadata, false, use.Facts, location(expressionToken(arg)))
			}
			return builtValue{}, fb.unsupported("slice argument whose type differs from its parameter", expressionToken(arg))
		}
		return value, nil
	}
	source, metadata, err := fb.referenceHolder(holder)
	if err != nil {
		return builtValue{}, err
	}
	sourceType, _ := fb.owner.module.Types.Lookup(source.typ)
	loc := location(holder.Token)
	if paramType.Kind == TypeSharedSlice {
		if sourceType.Kind == TypeMutableSlice && sourceType.Element == paramType.Element {
			use, ok := fb.owner.analyzer.ResolvedReferenceUseOf(holder)
			if !ok || use.Kind != sema.ReferenceUsePass || !use.Facts.Checkable() || use.Facts.MutableAuthority {
				return builtValue{}, fmt.Errorf("mutable slice argument %s has no resolved shared pass", holder.Value)
			}
			return fb.reborrowWholeSlice(source, metadata, false, use.Facts, loc)
		}
		if source.typ != param {
			return builtValue{}, fb.unsupported("passing a mutable slice holder to a shared slice parameter", holder.Token)
		}
		copied := cloneReferenceMetadata(metadata)
		copied.SourceBorrow = copied.Borrow
		value := fb.newValue(param, OwnershipSharedReference, loc)
		fb.emit(Operation{Kind: OpSliceCopyShared, Operands: []ValueID{source.id}, Results: []Value{value}, Reference: copied, Location: loc})
		fb.recordReference(value.ID, copied)
		return builtValue{id: value.ID, typ: param}, nil
	}
	if sourceType.Kind != TypeMutableSlice || source.typ != param {
		return builtValue{}, fb.unsupported("passing a shared slice holder to a mutable slice parameter", holder.Token)
	}
	use, ok := fb.owner.analyzer.ResolvedReferenceUseOf(holder)
	if !ok || use.Kind != sema.ReferenceUsePass {
		return builtValue{}, fmt.Errorf("mutable slice argument %s has no resolved pass", holder.Value)
	}
	return fb.reborrowWholeSlice(source, metadata, true, use.Facts, loc)
}

// reborrowWholeSlice reborrows the complete extent of a slice holder under the
// new borrow/lifetime of facts, keeping every other source fact: the holder's
// validity is checked, its length observed, the open range normalized and
// slice.reborrow-shared / slice.reborrow-mut emitted (the form of
// `ref source[..]`).
func (fb *functionBuilder) reborrowWholeSlice(source builtValue, metadata *ReferenceMetadata, mutable bool, facts sema.ReferenceFacts, loc Location) (builtValue, error) {
	sourceType, _ := fb.owner.module.Types.Lookup(source.typ)
	if mutable && sourceType.Kind != TypeMutableSlice {
		return builtValue{}, &UnsupportedFeatureError{Feature: "mutable reborrow of a shared slice", Package: fb.owner.maxPackage, Location: loc}
	}
	typ, err := fb.owner.module.Types.InternSlice(sourceType.Element, mutable)
	if err != nil {
		return builtValue{}, err
	}
	guard, err := fb.validateReference(source, metadata, loc)
	if err != nil {
		return builtValue{}, err
	}
	uintType := fb.owner.module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	extent := fb.result(Operation{Kind: OpSliceLength, Operands: []ValueID{source.id}, ReferenceGuard: guard, Location: loc}, uintType)
	start, length := fb.newValue(uintType, OwnershipImmediate, loc), fb.newValue(uintType, OwnershipImmediate, loc)
	fb.emit(Operation{Kind: OpSliceRangeNormalize, Operands: []ValueID{extent.id}, Results: []Value{start, length},
		SliceRange: &SliceRangeMetadata{ProofKind: SliceRangeProofOpenBound}, Location: loc})
	derived := cloneReferenceMetadata(metadata)
	derived.Borrow, derived.Lifetime, derived.SourceBorrow = uint32(facts.Borrow), uint32(facts.Lifetime), metadata.Borrow
	kind, ownership := OpSliceReborrowShared, OwnershipSharedReference
	if mutable {
		kind, ownership = OpSliceReborrowMutable, OwnershipMutableReference
	}
	value := fb.newValue(typ, ownership, loc)
	slice := SliceFacts{Element: sourceType.Element, Mutable: mutable, Reference: *derived, RuntimeLength: length.ID}
	fb.emit(Operation{Kind: kind, Operands: []ValueID{source.id, start.ID, length.ID}, Results: []Value{value},
		Reference: derived, Slice: &slice, ReferenceGuard: guard, Location: loc})
	fb.recordReference(value.ID, derived)
	return builtValue{id: value.ID, typ: typ}, nil
}

// internSignatureType interns a parameter or result type of a signature: a
// slice is the P16 slice type of its element and authority, any other type
// its ordinary Semantic IR type.
func (b *builder) internSignatureType(t sema.Type) (TypeID, error) {
	if b.maxPackage >= sliceLoweringPackage && sourceSliceType(t) {
		typeID, _, err := b.internSourceSliceType(t)
		return typeID, err
	}
	return b.internType(t)
}
