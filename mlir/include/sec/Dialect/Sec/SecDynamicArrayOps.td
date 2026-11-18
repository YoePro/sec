#ifndef SEC_DIALECT_SEC_DYNAMICARRAYOPS_TD
#define SEC_DIALECT_SEC_DYNAMICARRAYOPS_TD

// SEC-MLIR-P18 sections 81-96: schema-14 owning dynamic arrays and the
// general storage transitions. Every operation observes or changes the owner
// through its Place; capacity, backing and descriptor fields are never
// physical values a program can read. Allocation facts are the resolved Sema
// identities; no allocator, address or layout is selected here.

// Resolved backing facts (P18 sections 21-31, 56, 59). Identities are
// non-zero i32 values; vocabularies are verified by the operations.
defvar Sec_BackingFacts = (ins
    OptionalAttr<I32Attr>:$domain,
    OptionalAttr<StrAttr>:$transition,
    OptionalAttr<I32Attr>:$allocation,
    OptionalAttr<I32Attr>:$allocation_context,
    OptionalAttr<StrAttr>:$origin,
    OptionalAttr<StrAttr>:$relation,
    OptionalAttr<StrAttr>:$authority,
    OptionalAttr<I32Attr>:$reclamation,
    OptionalAttr<StrAttr>:$stability,
    OptionalAttr<StrAttr>:$space);

// Section 83: the canonical non-allocating empty owner.
def Sec_DynamicArrayEmptyOp : Sec_Op<"dynamic_array.empty", [Pure]> {
  let summary = "Create the empty owning dynamic array without allocation";
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

// Section 84: a candidate owner with at least `count` capacity and an
// established backing domain, usable only when `failed` is false.
def Sec_DynamicArrayAllocateOp : Sec_Op<"dynamic_array.allocate"> {
  let summary = "Total fallible allocation of an empty owner with capacity";
  let arguments = !con((ins StrAttr:$count), Sec_BackingFacts);
  let results = (outs AnyType:$array, I1:$failed, AnyType:$error);
  let hasVerifier = 1;
}

class Sec_DynamicArrayObserveOp<string mnemonic, string text>
    : Sec_Op<mnemonic, [NoMemoryEffect]> {
  let summary = text;
  let arguments = (ins AnyType:$owner);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}
// Sections 85-87: length as uint or proven int, and the core-internal capacity.
def Sec_DynamicArrayLenOp : Sec_DynamicArrayObserveOp<"dynamic_array.len",
    "Observe the runtime length as uint through the owner Place">;
def Sec_DynamicArrayLenIntOp : Sec_DynamicArrayObserveOp<"dynamic_array.len_int",
    "Observe the runtime length as int under the proven representability rule">;
def Sec_DynamicArrayCapacityInternalOp : Sec_DynamicArrayObserveOp<"dynamic_array.capacity_internal",
    "Observe the compiler/core-internal capacity; never ordinary member lookup">;
// Sections 63-67: descriptor-state tests of the growth protocol.
def Sec_DynamicArrayHasBackingOp : Sec_DynamicArrayObserveOp<"dynamic_array.has_backing",
    "Test whether the owner has an established backing domain">;

def Sec_DynamicArrayHasCapacityOp : Sec_Op<"dynamic_array.has_capacity", [NoMemoryEffect]> {
  let summary = "Test whether `count` more elements fit without growth";
  let arguments = (ins AnyType:$owner, StrAttr:$count);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

// Section 88: the total bounds predicate 0 <= index < length.
def Sec_DynamicArrayIndexInBoundsOp : Sec_Op<"dynamic_array.index_in_bounds", [NoMemoryEffect]> {
  let summary = "Total dynamic-array bounds predicate";
  let arguments = (ins AnyType:$owner, AnyType:$index, BoolAttr:$index_signed);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

// Section 89: the guarded element Place with the owner's authority and its
// current backing-domain dependency; no pointer arithmetic is exposed.
def Sec_DynamicArrayElementPlaceOp : Sec_Op<"dynamic_array.element_place"> {
  let summary = "Form a guarded dynamic-array element Place";
  let arguments = (ins AnyType:$owner, AnyType:$index, BoolAttr:$index_signed,
                       StrAttr:$bounds_kind, StrAttr:$bounds_proof, I32Attr:$backing_domain);
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}

// Section 90: initialize the element at the old length and increment the
// length after capacity is ensured; it never allocates.
def Sec_DynamicArrayAppendPreparedOp : Sec_Op<"dynamic_array.append_prepared"> {
  let summary = "Append into ensured capacity without allocation";
  let arguments = (ins AnyType:$owner, AnyType:$element);
  let hasVerifier = 1;
}

// Section 91: fallible backing establishment or growth with its static
// canonical transition; failure leaves the owner unchanged.
def Sec_DynamicArrayGrowBackingOp : Sec_Op<"dynamic_array.grow_backing"> {
  let summary = "Compiler/core-internal fallible backing growth";
  let arguments = !con((ins AnyType:$owner, StrAttr:$count), Sec_BackingFacts);
  let results = (outs I1:$failed, AnyType:$error);
  let hasVerifier = 1;
}

// Sections 92-96: general storage transitions of one owner's backing domain.
class Sec_StorageTransitionOp<string mnemonic, string text> : Sec_Op<mnemonic> {
  let summary = text;
  let arguments = !con((ins AnyType:$owner), Sec_BackingFacts);
  let hasVerifier = 1;
}
def Sec_StorageEstablishDomainOp : Sec_StorageTransitionOp<"storage.establish_domain",
    "Absent -> Live(initial epoch) with origin, relation, authority, stability and space">;
def Sec_StorageAdvanceEpochOp : Sec_StorageTransitionOp<"storage.advance_epoch",
    "Live(old) -> Live(new) for the same logical domain; invalidates old dependencies">;
def Sec_StorageEndDomainOp : Sec_StorageTransitionOp<"storage.end_domain",
    "Live -> Ended; no physical deallocation by itself">;
def Sec_StorageReclaimOp : Sec_StorageTransitionOp<"storage.reclaim",
    "Consume the resolved owning-value reclamation plan; never element destruction">;

#endif
