#ifndef SEC_DIALECT_SEC_DIALECT_TD
#define SEC_DIALECT_SEC_DIALECT_TD

include "mlir/IR/OpBase.td"

def Sec_Dialect : Dialect {
  let name = "sec";
  let summary = "Sec semantic lowering dialect";
  let description = [{
    The Sec dialect preserves semantic obligations that remain after validated
    Sec Semantic IR enters MLIR. Schema version 10 carries explicit target-plan
    metadata and high-level scalar, enum, union, struct, and fixed-array
    semantics while retaining all earlier schema constructs as compatibility
    inputs.
  }];
  let cppNamespace = "::sec";
  let useDefaultTypePrinterParser = 1;
  let useDefaultAttributePrinterParser = 1;
  let isExtensible = 0;
  let extraClassDeclaration = [{
    static constexpr unsigned kSchemaVersion = 10;
    void registerAttributes();
    void registerTypes();
    void registerOperations();
    ::llvm::LogicalResult verifyOperationAttribute(
        ::mlir::Operation *operation,
        ::mlir::NamedAttribute attribute) override;
  }];
}

#endif // SEC_DIALECT_SEC_DIALECT_TD
