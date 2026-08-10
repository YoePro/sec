#ifndef SEC_DIALECT_SEC_OPS_TD
#define SEC_DIALECT_SEC_OPS_TD

include "mlir/IR/OpBase.td"
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

def Sec_ConstFloatOp : Sec_Op<"const.float", [Pure]> {
  let summary = "Exact source spelling of a Sec floating constant";
  let arguments = (ins StrAttr:$lexeme);
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

def Sec_FailArithmeticOp : Sec_Op<"fail.arithmetic", [Terminator]> {
  let summary = "Deterministic non-returning Sec arithmetic failure";
  let arguments = (ins Variadic<AnyType>:$reasons,
                       OptionalAttr<StrAttr>:$category);
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

#endif // SEC_DIALECT_SEC_OPS_TD
