#ifndef SEC_DIALECT_SEC_DIALECT_TD
#define SEC_DIALECT_SEC_DIALECT_TD

include "mlir/IR/OpBase.td"

def Sec_Dialect : Dialect {
  let name = "sec";
  let summary = "Sec semantic lowering dialect";
  let description = [{
    The Sec dialect preserves semantic obligations that remain after validated
    Sec Semantic IR enters MLIR. Schema version 1 contains only nominal type
    identity primitives and common metadata conventions.
  }];
  let cppNamespace = "::sec";
  let useDefaultTypePrinterParser = 1;
  let isExtensible = 0;
  let extraClassDeclaration = [{
    static constexpr unsigned kSchemaVersion = 1;
    void registerTypes();
  }];
}

#endif // SEC_DIALECT_SEC_DIALECT_TD
