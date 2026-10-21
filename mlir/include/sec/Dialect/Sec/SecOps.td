#ifndef SEC_DIALECT_SEC_OPS_TD
#define SEC_DIALECT_SEC_OPS_TD

include "mlir/IR/OpBase.td"
include "mlir/IR/SymbolInterfaces.td"
include "mlir/Interfaces/SideEffectInterfaces.td"
include "sec/Dialect/Sec/SecDialect.td"

class Sec_Op<string mnemonic, list<Trait> traits = []>
    : Op<Sec_Dialect, mnemonic, traits>;

def Sec_ConstIntOp : Sec_Op<"const.int", [Pure]> {
  let summary = "Exact Sec integer constant";
  let arguments = (ins StrAttr:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ConstBoolOp : Sec_Op<"const.bool", [Pure]> {
  let summary = "Sec boolean constant";
  let arguments = (ins BoolAttr:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

// P20 §§ 18, 74: the frontend's canonical binary payload, rounded once from
// the exact source value, travels as `bits`; lowering never reparses the
// lexeme (P20 § 76).
def Sec_ConstFloatOp : Sec_Op<"const.float", [Pure]> {
  let summary = "Exact source spelling of a Sec floating constant";
  let arguments = (ins StrAttr:$lexeme, OptionalAttr<I64Attr>:$bits);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ConstDecimalOp : Sec_Op<"const.decimal", [Pure]> {
  let summary = "Exact Sec decimal constant";
  let arguments = (ins StrAttr:$coefficient, I32Attr:$scale,
                       StrAttr:$lexeme);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ConstStringOp : Sec_Op<"const.string", [Pure]> {
  let summary = "Decoded Sec string constant";
  let arguments = (ins StrAttr:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_StorageDeclareOp : Sec_Op<"storage.declare"> {
  let summary = "Declare one semantic storage identity";
  let results = (outs AnyType:$storage);
  let hasVerifier = 1;
}

def Sec_StorageInitOp : Sec_Op<"storage.init"> {
  let summary = "Initialize semantic storage";
  let arguments = (ins AnyType:$storage, AnyType:$value);
  let hasVerifier = 1;
}

def Sec_StorageLoadOp : Sec_Op<"storage.load", [Pure]> {
  let summary = "Load from semantic storage";
  let arguments = (ins AnyType:$storage);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_StorageStoreOp : Sec_Op<"storage.store"> {
  let summary = "Replace a trivial semantic storage value";
  let arguments = (ins AnyType:$storage, AnyType:$value);
  let hasVerifier = 1;
}

def Sec_CallDirectOp : Sec_Op<"call.direct"> {
  let summary = "Resolved direct Sec call";
  let arguments = (ins FlatSymbolRefAttr:$callee,
                       Variadic<AnyType>:$arguments);
  let results = (outs Variadic<AnyType>:$results);
  let hasVerifier = 1;
}

def Sec_CallForeignOp : Sec_Op<"call.foreign"> {
  let summary = "Resolved foreign Sec call";
  let arguments = (ins FlatSymbolRefAttr:$callee,
                       Variadic<AnyType>:$arguments);
  let results = (outs Variadic<AnyType>:$results);
  let hasVerifier = 1;
}

def Sec_IntUnaryPlusOp : Sec_Op<"int.unary_plus", [Pure]> {
  let summary = "Resolved Sec integer unary plus";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_IntNegCheckedOp : Sec_Op<"int.neg_checked", [Pure]> {
  let summary = "Total checked Sec signed integer negation";
  let arguments = (ins AnyType:$value);
  let results = (outs Variadic<AnyType>:$results);
  let hasVerifier = 1;
}

def Sec_IntBinaryCheckedOp : Sec_Op<"int.binary_checked", [Pure]> {
  let summary = "Total checked Sec integer binary arithmetic";
  let arguments = (ins StrAttr:$kind, AnyType:$left, AnyType:$right);
  let results = (outs Variadic<AnyType>:$results);
  let hasVerifier = 1;
}

def Sec_IntBitNotOp : Sec_Op<"int.bit_not", [Pure]> {
  let summary = "Resolved Sec integer bitwise complement";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_IntBitwiseOp : Sec_Op<"int.bitwise", [Pure]> {
  let summary = "Resolved Sec integer bitwise binary operation";
  let arguments = (ins StrAttr:$kind, AnyType:$left, AnyType:$right);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_IntShiftCheckedOp : Sec_Op<"int.shift_checked", [Pure]> {
  let summary = "Total checked Sec integer shift";
  let arguments = (ins StrAttr:$kind, AnyType:$value, AnyType:$count);
  let results = (outs Variadic<AnyType>:$results);
  let hasVerifier = 1;
}

def Sec_IntCmpOp : Sec_Op<"int.cmp", [Pure]> {
  let summary = "Resolved Sec integer comparison";
  let arguments = (ins StrAttr:$predicate, AnyType:$left, AnyType:$right);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

// P20 floating operations (sec-mlir-dialect_package20.md §§ 20–26, 63–65,
// 73, 75, 77), admitted early in schema 11 for float, float32 and float64:
// nearest-even rounding, no fast-math, no failure edge; `rem` is the
// truncation remainder.
def Sec_FloatNegOp : Sec_Op<"float.neg", [Pure]> {
  let summary = "Sec floating negation (toggles the sign, NaN stays NaN)";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_FloatBinaryOp : Sec_Op<"float.binary", [Pure]> {
  let summary = "Sec floating arithmetic: add, sub, mul, div or rem";
  let arguments = (ins StrAttr:$kind, AnyType:$left, AnyType:$right);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_FloatCmpOp : Sec_Op<"float.cmp", [Pure]> {
  let summary = "Sec floating comparison (ordered except ne, NaN-aware)";
  let arguments = (ins StrAttr:$predicate, AnyType:$left, AnyType:$right);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

// P20 NumericConvertExactOp (sec-mlir-dialect_package20.md §§ 44, 63, 73),
// admitted early in schema 11 for built-in integer conversions the frontend
// proved exact; P20 extends it to the float family (MD-104).
def Sec_NumericConvertExactOp : Sec_Op<"numeric.convert_exact", [Pure]> {
  let summary = "Exact, infallible numeric conversion proven before lowering";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_FailArithmeticOp : Sec_Op<"fail.arithmetic", [Terminator]> {
  let summary = "Deterministic non-returning Sec arithmetic failure";
  let arguments = (ins Variadic<AnyType>:$reasons,
                       OptionalAttr<StrAttr>:$category);
  let hasVerifier = 1;
}

def Sec_UnreachableOp : Sec_Op<"unreachable", [Terminator]> {
  let summary = "Compiler-proven unreachable Sec control-flow path";
  let arguments = (ins StrAttr:$reason);
  let hasVerifier = 1;
}

def Sec_ArithmeticFailureReasonConstantOp
    : Sec_Op<"arithmetic_failure_reason.constant", [Pure]> {
  let summary = "Constant typed arithmetic failure reason";
  let arguments = (ins StrAttr:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ArithmeticErrorFromReasonOp
    : Sec_Op<"arithmetic_error.from_reason", [Pure]> {
  let summary = "Map a non-none reason to core ArithmeticError";
  let arguments = (ins AnyType:$reason);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ResultOkOp : Sec_Op<"result.ok", [Pure]> {
  let summary = "Construct semantic Result success";
  let arguments = (ins Variadic<AnyType>:$payload);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ResultErrOp : Sec_Op<"result.err", [Pure]> {
  let summary = "Construct semantic Result error";
  let arguments = (ins Variadic<AnyType>:$payload);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ResultIsErrOp : Sec_Op<"result.is_err", [Pure]> {
  let summary = "Test whether a semantic Result contains Err";
  let arguments = (ins AnyType:$value);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

def Sec_ResultUnwrapOkOp : Sec_Op<"result.unwrap_ok", [Pure]> {
  let summary = "Project a Result success on its proven Ok path";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ResultUnwrapErrOp : Sec_Op<"result.unwrap_err", [Pure]> {
  let summary = "Project a Result error on its proven Err path";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_CoreErrorIsVariantOp : Sec_Op<"core_error.is_variant", [Pure]> {
  let summary = "Test a compiler-known core error variant";
  let arguments = (ins StrAttr:$variant, AnyType:$value);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

def Sec_EnumConstantOp : Sec_Op<"enum.constant", [Pure]> {
  let summary = "Construct one declared Sec enum case";
  let arguments = (ins I32Attr:$caseOrdinal);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_EnumFromIntegerOp : Sec_Op<"enum.from_integer", [Pure]> {
  let summary = "Explicit validated integer-to-enum conversion";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_EnumToIntegerOp : Sec_Op<"enum.to_integer", [Pure]> {
  let summary = "Explicit validated enum-to-integer conversion";
  let arguments = (ins AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_EnumCmpOp : Sec_Op<"enum.cmp", [Pure]> {
  let summary = "Same-enum numeric equality comparison";
  let arguments = (ins StrAttr:$predicate, AnyType:$left, AnyType:$right);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

def Sec_UnionConstructOp : Sec_Op<"union.construct", [Pure]> {
  let summary = "Construct a high-level union semantic value";
  let arguments = (ins I32Attr:$variant,
                       ArrayAttr:$payloadActions,
                       ArrayAttr:$fieldNames,
                       Variadic<AnyType>:$payloads);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_UnionIsVariantOp : Sec_Op<"union.is_variant", [Pure]> {
  let summary = "Test the active semantic union variant";
  let arguments = (ins I32Attr:$variant, AnyType:$value);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

def Sec_UnionUnwrapPayloadOp : Sec_Op<"union.unwrap_payload", [Pure]> {
  let summary = "Project a single payload on its proven variant path";
  let arguments = (ins I32Attr:$variant, StrAttr:$payloadAction,
                       AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_UnionUnwrapFieldOp : Sec_Op<"union.unwrap_field", [Pure]> {
  let summary = "Project a named payload field on its proven variant path";
  let arguments = (ins I32Attr:$variant, StrAttr:$field,
                       StrAttr:$payloadAction, AnyType:$value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_StructConstructOp : Sec_Op<"struct.construct", [Pure]> {
  let summary = "Construct a fully initialized high-level Sec struct value";
  let arguments = (ins ArrayAttr:$field_origins, ArrayAttr:$field_actions,
                       Variadic<AnyType>:$fields);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_StructSpreadFieldsOp : Sec_Op<"struct.spread_fields", [Pure]> {
  let summary = "Project every stored field from one evaluated struct source";
  let arguments = (ins ArrayAttr:$actions, AnyType:$source);
  let results = (outs Variadic<AnyType>:$fields);
  let hasVerifier = 1;
}

def Sec_StructExtractOp : Sec_Op<"struct.extract", [Pure]> {
  let summary = "Project one resolved stored field from a Sec struct";
  let arguments = (ins I32Attr:$field, StrAttr:$action, AnyType:$source);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_StructReplaceFieldOp : Sec_Op<"struct.replace_field", [Pure]> {
  let summary = "Functionally replace one resolved field in a trivial Sec struct";
  let arguments = (ins I32Attr:$field, AnyType:$source, AnyType:$replacement);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ArrayConstructOp : Sec_Op<"array.construct", [Pure]> {
  let summary = "Construct a fixed Sec array from compact ordered segments";
  let arguments = (ins ArrayAttr:$segment_kinds,
                       ArrayAttr:$segment_lengths,
                       ArrayAttr:$segment_actions,
                       Variadic<AnyType>:$segments);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ArrayDefaultOp : Sec_Op<"array.default", [Pure]> {
  let summary = "Construct the canonical semantic default of a fixed Sec array";
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ArrayLenOp : Sec_Op<"array.len", [Pure]> {
  let summary = "Return the exact compile-time length of a fixed Sec array";
  let arguments = (ins AnyType:$array);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ArrayIndexInBoundsOp : Sec_Op<"array.index_in_bounds", [Pure]> {
  let summary = "Test a resolved integer index against fixed Sec array bounds";
  let arguments = (ins BoolAttr:$index_signed, AnyType:$array,
                       AnyType:$index);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

def Sec_ArrayExtractOp : Sec_Op<"array.extract", [Pure]> {
  let summary = "Copy one proven or guarded trivial fixed-array element";
  let arguments = (ins StrAttr:$bounds_kind, StrAttr:$bounds_proof,
                       StrAttr:$action, AnyType:$array, AnyType:$index);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_ArrayReplaceOp : Sec_Op<"array.replace", [Pure]> {
  let summary = "Functionally replace one proven or guarded fixed-array element";
  let arguments = (ins StrAttr:$bounds_kind, StrAttr:$bounds_proof,
                       AnyType:$array, AnyType:$index, AnyType:$new_value);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

// Canonical panic.bounds; schema 12 adds slice-index/slice-range provenance.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 23.
def Sec_FailBoundsOp : Sec_Op<"fail.bounds", [Terminator]> {
  let summary = "Deterministic non-returning Sec array or slice bounds failure";
  let hasVerifier = 1;
}

// ---------------------------------------------------------------------------
// Schema 11 (SEC-MLIR-P15): compiler-internal Places and direct safe
// references. Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md
// sections 6-29; rules/mlir/packages/sec-mlir-dialect_package15.md
// sections 63-82. Address stability replaces the withdrawn relocation class
// (package15.md section 28 revised 2026-10-03, storage.md section 14).
// Guard dominance, borrow semantics and escapes are checked by the
// --sec-verify-places, --sec-verify-reference-guards and
// --sec-verify-borrow-semantics passes.

defvar Sec_PlaceRootMetadata = (ins I32Attr:$place_root_id,
                                    I32Attr:$storage_identity,
                                    StrAttr:$address_space);

defvar Sec_ReferenceMetadata = (ins I32Attr:$borrow_id,
                                    I32Attr:$lifetime_id,
                                    StrAttr:$validity_policy,
                                    OptionalAttr<StrAttr>:$validity_proof,
                                    OptionalAttr<I32Attr>:$invalidation_domain,
                                    OptionalAttr<I32Attr>:$epoch_dependency,
                                    StrAttr:$address_stability,
                                    StrAttr:$provenance,
                                    OptionalAttr<I32Attr>:$source_borrow);

def Sec_PlaceValueOp : Sec_Op<"place.value", [Pure]> {
  let summary = "Read-only Place root over an immutable SSA value";
  let arguments = !con(Sec_PlaceRootMetadata, (ins AnyType:$value));
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}

def Sec_PlaceStorageOp : Sec_Op<"place.storage", [Pure]> {
  let summary = "Place root over address-taken semantic storage";
  let arguments = !con(Sec_PlaceRootMetadata, (ins AnyType:$storage));
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}

def Sec_PlaceFieldOp : Sec_Op<"place.field", [Pure]> {
  let summary = "Project a stored struct field Place";
  let arguments = (ins I32Attr:$field, AnyType:$base);
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}

def Sec_PlaceArrayIndexInBoundsOp : Sec_Op<"place.array_index_in_bounds", [Pure]> {
  let summary = "Test an index against the bounds of a fixed-array Place";
  let arguments = (ins BoolAttr:$index_signed, AnyType:$base, AnyType:$index);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

def Sec_PlaceArrayElementOp : Sec_Op<"place.array_element", [Pure]> {
  let summary = "Project a proven or guarded fixed-array element Place";
  let arguments = (ins StrAttr:$bounds_kind, StrAttr:$bounds_proof,
                       AnyType:$base, AnyType:$index);
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}

def Sec_PlaceUnionPayloadOp : Sec_Op<"place.union_payload", [Pure]> {
  let summary = "Project the payload Place of the active union or Result variant";
  let arguments = (ins I32Attr:$variant, AnyType:$base);
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}

def Sec_PlaceDerefOp : Sec_Op<"place.deref", [Pure]> {
  let summary = "Place denoted by a direct safe reference";
  let arguments = (ins AnyType:$reference);
  let results = (outs AnyType:$place);
  let hasVerifier = 1;
}

def Sec_PlaceReadOp : Sec_Op<"place.read"> {
  let summary = "Copy-trivial read of the value at a Place";
  let arguments = (ins StrAttr:$action, AnyType:$place);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
}

def Sec_PlaceWriteOp : Sec_Op<"place.write"> {
  let summary = "Trivial replacement of the value at a read/write Place";
  let arguments = (ins AnyType:$place, AnyType:$value);
  let hasVerifier = 1;
}

def Sec_RefBorrowSharedOp : Sec_Op<"ref.borrow_shared"> {
  let summary = "Borrow a shared direct reference to a Place";
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$place));
  let results = (outs AnyType:$reference);
  let hasVerifier = 1;
}

def Sec_RefBorrowMutOp : Sec_Op<"ref.borrow_mut"> {
  let summary = "Borrow an exclusive mutable direct reference to a read/write Place";
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$place));
  let results = (outs AnyType:$reference);
  let hasVerifier = 1;
}

def Sec_RefCopySharedOp : Sec_Op<"ref.copy_shared"> {
  let summary = "Copy a shared direct reference value";
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$source));
  let results = (outs AnyType:$reference);
  let hasVerifier = 1;
}

def Sec_RefMoveOp : Sec_Op<"ref.move"> {
  let summary = "Move a mutable direct reference value";
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$source));
  let results = (outs AnyType:$reference);
  let hasVerifier = 1;
}

def Sec_RefReborrowSharedOp : Sec_Op<"ref.reborrow_shared"> {
  let summary = "Derive a shared reborrow from a reference or a dereferenced sub-Place";
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$source));
  let results = (outs AnyType:$reference);
  let hasVerifier = 1;
}

def Sec_RefReborrowMutOp : Sec_Op<"ref.reborrow_mut"> {
  let summary = "Derive a mutable reborrow from mutable authority";
  let arguments = !con(Sec_ReferenceMetadata, (ins AnyType:$source));
  let results = (outs AnyType:$reference);
  let hasVerifier = 1;
}

// Schema 12 reuses the direct-reference predicate for shared/mutable slices.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 21.
def Sec_RefIsValidOp : Sec_Op<"ref.is_valid"> {
  let summary = "Total abstract runtime-validity predicate of a dynamic-epoch reference";
  let arguments = (ins AnyType:$reference);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

def Sec_RefCompareOp : Sec_Op<"ref.compare", [Pure]> {
  let summary = "Semantic equality of live storage identity and referenced location";
  let arguments = (ins StrAttr:$predicate, AnyType:$lhs, AnyType:$rhs);
  let results = (outs I1:$result);
  let hasVerifier = 1;
}

// Schema 12 reuses the borrow-end marker for shared/mutable slice holders.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 22.
def Sec_RefEndBorrowOp : Sec_Op<"ref.end_borrow"> {
  let summary = "Compile-time semantic end of a borrow";
  let arguments = (ins I32Attr:$borrow_id, I32Attr:$lifetime_id,
                       AnyType:$reference);
  let hasVerifier = 1;
}

// Static-duration storage (semantic_ir.md § 28; static.md): one module-level
// location with an exact compile-time initial value ("true"/"false" for bool,
// a decimal integer otherwise), read and written by symbol. Lowered to an LLVM
// global by sec-lower-static-storage (user decision 2026-10-18).
def Sec_StaticGlobalOp : Sec_Op<"static.global", [Symbol]> {
  let summary = "Module-level Sec static storage location";
  let arguments = (ins SymbolNameAttr:$sym_name, TypeAttr:$type,
                       StrAttr:$initial, BoolAttr:$is_mutable);
  let hasVerifier = 1;
}

def Sec_StaticLoadOp : Sec_Op<"static.load"> {
  let summary = "Read a Sec static storage location";
  let arguments = (ins FlatSymbolRefAttr:$global);
  let results = (outs AnyType:$result);
}

def Sec_StaticStoreOp : Sec_Op<"static.store"> {
  let summary = "Write a mutable Sec static storage location";
  let arguments = (ins FlatSymbolRefAttr:$global, AnyType:$value);
}

// Explicit panic, failed assertion or reached checked unreachable: a
// deterministic non-returning failure carrying the registered panic reason
// and the source's static message (semantic_ir.md §§ 55–57). Lowered like the
// other failures to the minimal panic endpoint (MD-101).
def Sec_FailPanicOp : Sec_Op<"fail.panic", [Terminator]> {
  let summary = "Deterministic non-returning explicit Sec panic";
  let arguments = (ins StrAttr:$reason, OptionalAttr<StrAttr>:$message);
  let hasVerifier = 1;
}

def Sec_FailReferenceGenerationOp : Sec_Op<"fail.reference_generation", [Terminator]> {
  let summary = "Deterministic invalid-reference-generation failure";
  let arguments = (ins StrAttr:$reason);
  let hasVerifier = 1;
}

include "sec/Dialect/Sec/SecSliceOps.td"

#endif // SEC_DIALECT_SEC_OPS_TD
