package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

// emitSlice maps verified P16 operations directly to schema 12, preserving
// original SSA operands, normalized tuples, reference facts and source locations.
// Guards stay in the existing CFG; no layout, implicit copy or runtime is chosen.
// Rules: rules/mlir/sec_mlir_lowering.md — v12.1–21,29–31;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§87,91–103.
func (e *emitter) emitSlice(function *semantic.Function, op semantic.Operation, values map[semantic.ValueID]semantic.Value) error {
	if e.schemaVersion < 12 {
		return &UnsupportedLoweringError{Feature: "slice operations require schema 12", Function: function.ID}
	}
	if op.Reference != nil {
		switch op.Kind {
		case semantic.OpSliceBorrowShared, semantic.OpSliceBorrowMutable,
			semantic.OpSliceReborrowShared, semantic.OpSliceReborrowMutable,
			semantic.OpSliceCopyShared, semantic.OpSliceMoveMutable, semantic.OpSliceElementPlace,
			semantic.OpStringViewBorrow:
		default:
			return &UnsupportedLoweringError{Feature: "reference properties on " + string(op.Kind), Function: function.ID}
		}
	}
	if op.SliceRange != nil && op.Kind != semantic.OpSliceRangeNormalize && op.Kind != semantic.OpSliceRangeCheck {
		return &UnsupportedLoweringError{Feature: "range properties on " + string(op.Kind), Function: function.ID}
	}
	if err := validateReferenceFacts(op.Reference, function.ID); err != nil {
		return err
	}
	names := map[semantic.OpKind]string{
		semantic.OpSliceRangeNormalize: "range_normalize", semantic.OpSliceRangeCheck: "range_check",
		semantic.OpSliceBorrowShared: "borrow_shared", semantic.OpSliceBorrowMutable: "borrow_mut",
		semantic.OpSliceReborrowShared: "reborrow_shared", semantic.OpSliceReborrowMutable: "reborrow_mut",
		semantic.OpSliceCopyShared: "copy_shared", semantic.OpSliceMoveMutable: "move_mut",
		semantic.OpSliceLength: "len", semantic.OpSliceIndexInBounds: "index_in_bounds",
		semantic.OpSliceElementPlace: "element_place",
	}
	// MD-126: construction from a string Place and its rune count are string
	// operations; length, copy and reborrow of a view reuse the shared
	// bounded-view slice operations over !sec.string_view.
	mnemonic := "sec.slice." + names[op.Kind]
	switch {
	case op.Kind == semantic.OpStringRuneLength:
		mnemonic = "sec.string.rune_len"
	case op.Kind == semantic.OpStringViewBorrow:
		mnemonic = "sec.string.view_borrow"
	case op.Kind == semantic.OpStringViewDecodeNext:
		mnemonic = "sec.string_view.decode_next"
	case op.Kind == semantic.OpStringViewByteLength:
		mnemonic = "sec.string_view.byte_len"
	}
	var properties []string
	if r := op.SliceRange; r != nil {
		properties = append(properties, fmt.Sprintf("start_present = %t, end_present = %t, end_exclusive = %t, start_signed = %t, end_signed = %t", r.StartPresent, r.EndPresent, r.EndExclusive, r.StartSigned, r.EndSigned))
		if op.Kind == semantic.OpSliceRangeNormalize {
			properties = append(properties, "proof_kind = "+mlirString(string(r.ProofKind)))
		}
	}
	if op.Reference != nil {
		properties = append(properties, referenceProperties(op.Reference, op.Reference.SourceBorrow != 0))
	}
	if op.Kind == semantic.OpSliceIndexInBounds || op.Kind == semantic.OpSliceElementPlace {
		properties = append(properties, fmt.Sprintf("index_signed = %t", op.ArrayIndexSigned))
	}
	if op.Kind == semantic.OpSliceElementPlace {
		properties = append(properties, "bounds_kind = "+mlirString(string(op.ArrayCheckKind)), "bounds_proof = "+mlirString(string(op.ArrayProofKind)))
	}
	if err := e.emitResults(op); err != nil {
		return err
	}
	operands := make([]string, len(op.Operands))
	for i, operand := range op.Operands {
		operands[i] = valueName(operand)
	}
	fmt.Fprintf(&e.out, "\"%s\"(%s) ", mnemonic, strings.Join(operands, ", "))
	if len(properties) != 0 {
		fmt.Fprintf(&e.out, "<{%s}> ", strings.Join(properties, ", "))
	}
	extras := operationAttributes(op, false)
	if op.Reference != nil {
		extras = append(extras, referenceExtras(op.Reference, "sec.")...)
	}
	if facts := op.Slice; facts != nil {
		if facts.StaticRange != nil {
			extras = append(extras, "sec.slice_static_start = "+mlirString(facts.StaticRange.Start.String()), "sec.slice_static_end_exclusive = "+mlirString(facts.StaticRange.EndExclusive.String()))
		}
		if facts.DynamicRange != 0 {
			extras = append(extras, fmt.Sprintf("sec.slice_range_id = %d : i32", facts.DynamicRange))
		}
	}
	if len(extras) != 0 {
		fmt.Fprintf(&e.out, "{%s} ", strings.Join(extras, ", "))
	}
	return e.finishTypedOperation(op, values)
}

// isStringView reports whether id is the MD-126 StringView type.
func (e *emitter) isStringView(id semantic.TypeID) bool {
	typ, ok := e.module.Types.Lookup(id)
	return ok && typ.Kind == semantic.TypeStringView
}
