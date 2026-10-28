#ifndef SEC_DIALECT_SEC_ATTRIBUTES_TD
#define SEC_DIALECT_SEC_ATTRIBUTES_TD

include "mlir/IR/AttrTypeBase.td"
include "sec/Dialect/Sec/SecDialect.td"

class Sec_Attr<string name, string attrMnemonic>
    : AttrDef<Sec_Dialect, name> {
  let mnemonic = attrMnemonic;
}

def Sec_EnumCaseAttr : Sec_Attr<"EnumCase", "enum_case"> {
  let summary = "Declared enum case identity and arbitrary-precision value";
  let parameters = (ins
    "uint32_t":$ordinal,
    "::mlir::StringAttr":$name,
    "::mlir::StringAttr":$value
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

def Sec_UnionFieldAttr : Sec_Attr<"UnionField", "union_field"> {
  let summary = "Named field in a struct-like union payload";
  let parameters = (ins
    "::mlir::StringAttr":$name,
    "::mlir::Type":$type
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

def Sec_UnionVariantAttr : Sec_Attr<"UnionVariant", "union_variant"> {
  let summary = "Stable union variant identity and semantic payload shape";
  let parameters = (ins
    "uint32_t":$index,
    "::mlir::StringAttr":$name,
    "::mlir::StringAttr":$kind,
    OptionalParameter<"::mlir::Type">:$payload,
    "::mlir::ArrayAttr":$fields
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

def Sec_StructTagAttr : Sec_Attr<"StructTag", "struct_tag"> {
  let summary = "Source-order metadata attached to a stored Sec struct field";
  let parameters = (ins
    "::mlir::StringAttr":$key,
    "::mlir::StringAttr":$value
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

def Sec_StructFieldAttr : Sec_Attr<"StructField", "struct_field"> {
  let summary = "Declaration-order identity and metadata for a stored Sec struct field";
  let parameters = (ins
    "uint32_t":$ordinal,
    "::mlir::StringAttr":$name,
    "::mlir::Type":$type,
    "::mlir::ArrayAttr":$tags
  );
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

// MD-114 §§2–3,7: one typed module registry and independent field-write proof.
def Sec_RegisterFieldAttr : Sec_Attr<"RegisterField", "register_field"> {
  let parameters = (ins "uint32_t":$ordinal, "::mlir::StringAttr":$name,
    "uint32_t":$offset, "uint32_t":$width, "::mlir::Type":$type,
    "::mlir::StringAttr":$access, "bool":$shadow,
    OptionalParameter<"::mlir::StringAttr">:$unit, "bool":$reserved);
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}
def Sec_RegisterDefAttr : Sec_Attr<"RegisterDef", "register_def"> {
  let parameters = (ins "::mlir::StringAttr":$identity, "uint32_t":$width,
    "::mlir::StringAttr":$order,
    OptionalParameter<"::mlir::StringAttr">:$byte_order,
    "::mlir::ArrayAttr":$fields);
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}
def Sec_RegisterWriteProofAttr : Sec_Attr<"RegisterWriteProof", "register_write_proof"> {
  let parameters = (ins "::mlir::StringAttr":$basis,
    OptionalParameter<"::mlir::StringAttr">:$low,
    OptionalParameter<"::mlir::StringAttr">:$high);
  let assemblyFormat = "`<` struct(params) `>`";
  let genVerifyDecl = 1;
}

#endif // SEC_DIALECT_SEC_ATTRIBUTES_TD
