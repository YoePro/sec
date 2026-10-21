#ifndef SEC_DIALECT_SEC_SLICEOPS_TD
#define SEC_DIALECT_SEC_SLICEOPS_TD

// P16 sections 91-103: high-level bounded-reference operations. Length and
// provenance are values/metadata, never descriptor layout or type parameters.
defvar Sec_SliceRangeOperands = (ins AnyType:$source_length, Variadic<AnyType>:$bounds,
    BoolAttr:$start_present, BoolAttr:$end_present, BoolAttr:$end_exclusive,
    BoolAttr:$start_signed, BoolAttr:$end_signed);
// No memory effects does not grant unrestricted speculation: a normalization
// may depend on a dominating proof. Slice observations likewise stay behind
// their source-validity guard (P16 sections 45-46,105-110).
class Sec_SliceRangeOp<string mnemonic> : Sec_Op<mnemonic, [NoMemoryEffect]> {
  let arguments = Sec_SliceRangeOperands;
  let hasVerifier = 1;
}
def Sec_SliceRangeNormalizeOp : Sec_SliceRangeOp<"slice.range_normalize"> {
  let summary = "Normalize a proven-safe range in its original integer carriers";
  let arguments = !con(Sec_SliceRangeOperands, (ins StrAttr:$proof_kind));
  let results = (outs AnyType:$start, AnyType:$length);
}
def Sec_SliceRangeCheckOp : Sec_SliceRangeOp<"slice.range_check"> {
  let summary = "Total range validation with normalized uints and canonical RangeError";
  let results = (outs AnyType:$start, AnyType:$length, I1:$failed, AnyType:$error);
}

class Sec_SliceBorrowOp<string mnemonic> : Sec_Op<mnemonic> {
  let arguments = !con(Sec_ReferenceMetadata,
    (ins AnyType:$source, AnyType:$start, AnyType:$length));
  let results = (outs AnyType:$slice);
  let hasVerifier = 1;
}
def Sec_SliceBorrowSharedOp : Sec_SliceBorrowOp<"slice.borrow_shared">;
def Sec_SliceBorrowMutOp : Sec_SliceBorrowOp<"slice.borrow_mut">;
def Sec_SliceReborrowSharedOp : Sec_SliceBorrowOp<"slice.reborrow_shared">;
def Sec_SliceReborrowMutOp : Sec_SliceBorrowOp<"slice.reborrow_mut">;

class Sec_SliceTransferOp<string mnemonic> : Sec_Op<mnemonic> {
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$source));
  let results = (outs AnyType:$slice);
  let hasVerifier = 1;
}
def Sec_SliceCopySharedOp : Sec_SliceTransferOp<"slice.copy_shared">;
def Sec_SliceMoveMutOp : Sec_SliceTransferOp<"slice.move_mut">;

def Sec_SliceLenOp : Sec_Op<"slice.len"> {
  let summary = "Observe an unsigned element count without reading elements";
  let arguments = (ins AnyType:$source);
  let results = (outs AnyType:$length);
  let hasVerifier = 1;
}
def Sec_SliceIndexInBoundsOp : Sec_Op<"slice.index_in_bounds", [NoMemoryEffect]> {
  let arguments = (ins AnyType:$source, AnyType:$index, BoolAttr:$index_signed);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}
def Sec_SliceElementPlaceOp : Sec_Op<"slice.element_place"> {
  let summary = "Form a proven or guarded element Place with source authority";
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$source, AnyType:$index,
    BoolAttr:$index_signed, StrAttr:$bounds_kind, StrAttr:$bounds_proof));
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}
#endif
