#ifndef SEC_DIALECT_SEC_TYPES_TD
#define SEC_DIALECT_SEC_TYPES_TD

include "mlir/IR/AttrTypeBase.td"
include "sec/Dialect/Sec/SecDialect.td"

class Sec_Type<string name, string typeMnemonic>
    : TypeDef<Sec_Dialect, name> {
  let mnemonic = typeMnemonic;
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
