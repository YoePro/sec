#ifndef SEC_DIALECT_SEC_TYPES_TD
#define SEC_DIALECT_SEC_TYPES_TD

include "mlir/IR/AttrTypeBase.td"
include "sec/Dialect/Sec/SecDialect.td"
include "sec/Dialect/Sec/SecAttributes.td"

class Sec_Type<string name, string typeMnemonic>
    : TypeDef<Sec_Dialect, name> {
  let mnemonic = typeMnemonic;
}

class Sec_SingletonType<string name, string typeMnemonic>
    : Sec_Type<name, typeMnemonic> {
  let assemblyFormat = "";
}

def Sec_IntType : Sec_SingletonType<"Int", "int"> {
  let summary = "Sec target-sized signed integer";
}

def Sec_UIntType : Sec_SingletonType<"UInt", "uint"> {
  let summary = "Sec target-sized unsigned integer";
}

def Sec_FloatType : Sec_SingletonType<"Float", "float"> {
  let summary = "Sec default semantic floating-point type";
}

def Sec_CharType : Sec_SingletonType<"Char", "char"> {
  let summary = "Sec character type";
}

def Sec_RuneType : Sec_SingletonType<"Rune", "rune"> {
  let summary = "Sec Unicode scalar type";
}

def Sec_StringType : Sec_SingletonType<"String", "string"> {
  let summary = "Sec high-level string type";
}

def Sec_ArithmeticFailureReasonType
    : Sec_SingletonType<"ArithmeticFailureReason", "arithmetic_failure_reason"> {
  let summary = "Typed reason produced by checked Sec integer arithmetic";
}

def Sec_CoreErrorType : Sec_Type<"CoreError", "core_error"> {
  let summary = "Compiler-known core error identity without physical layout";
  let parameters = (ins "::mlir::StringAttr":$identity);
  let assemblyFormat = "`<` $identity `>`";
  let genVerifyDecl = 1;
}

def Sec_ResultType : Sec_Type<"Result", "result"> {
  let summary = "Semantic Sec Result success and error identity";
  let parameters = (ins "::mlir::Type":$successType,
                         "::mlir::Type":$errorType);
  let assemblyFormat = "`<` $successType `,` $errorType `>`";
  let genVerifyDecl = 1;
}

def Sec_EnumType : Sec_Type<"Enum", "enum"> {
  let summary = "Nominal Sec enum with semantic underlying representation";
  let parameters = (ins
    "::mlir::StringAttr":$identity,
    "::mlir::Type":$underlying,
    "::mlir::StringAttr":$representation,
    "uint32_t":$bitWidth,
    "::mlir::ArrayAttr":$cases
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

def Sec_UnionType : Sec_Type<"Union", "union"> {
  let summary = "Nominal Sec tagged union without selected physical layout";
  let parameters = (ins
    "::mlir::StringAttr":$identity,
    "::mlir::ArrayAttr":$typeArguments,
    "::mlir::ArrayAttr":$variants
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

def Sec_StructType : Sec_Type<"Struct", "struct"> {
  let summary = "Nominal Sec struct semantic value without physical layout";
  let parameters = (ins
    "::mlir::StringAttr":$identity,
    "::mlir::ArrayAttr":$typeArguments,
    "::mlir::ArrayAttr":$fields
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

def Sec_ArrayType : Sec_Type<"Array", "array"> {
  let summary = "Fixed Sec array semantic value without physical layout";
  let parameters = (ins
    "::mlir::Type":$elementType,
    "::mlir::StringAttr":$length
  );
  let assemblyFormat = "`<` $elementType `,` $length `>`";
  let genVerifyDecl = 1;
}

def Sec_DecimalType : Sec_SingletonType<"Decimal", "decimal"> {
  let summary = "Sec exact decimal type";
}

def Sec_Decimal128Type : Sec_SingletonType<"Decimal128", "decimal128"> {
  let summary = "Sec exact decimal type with a 128-bit coefficient family";
}

def Sec_NeverType : Sec_SingletonType<"Never", "never"> {
  let summary = "Sec never type";
}

def Sec_StorageType : Sec_Type<"Storage", "storage"> {
  let summary = "Sec semantic storage identity";
  let parameters = (ins "::mlir::Type":$elementType);
  let assemblyFormat = "`<` $elementType `>`";
  let genVerifyDecl = 1;
}

def Sec_NamedType : Sec_Type<"Named", "named"> {
  let summary = "Sec named type identity over a selected base representation";
  let parameters = (ins
    "::mlir::StringAttr":$identity,
    "::mlir::Type":$base
  );
  let assemblyFormat = "`<` $identity `,` $base `>`";
  let genVerifyDecl = 1;
}

def Sec_DistinctType : Sec_Type<"Distinct", "distinct"> {
  let summary = "Sec distinct type identity over a selected base representation";
  let parameters = (ins
    "::mlir::StringAttr":$identity,
    "::mlir::Type":$base
  );
  let assemblyFormat = "`<` $identity `,` $base `>`";
  let genVerifyDecl = 1;
}

#endif // SEC_DIALECT_SEC_TYPES_TD
