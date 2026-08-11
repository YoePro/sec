#ifndef SEC_DIALECT_SEC_DIALECT_TD
#define SEC_DIALECT_SEC_DIALECT_TD

include "mlir/IR/OpBase.td"

def Sec_Dialect : Dialect {
  let name = "sec";
  let summary = "Sec semantic lowering dialect";
  let description = [{
    The Sec dialect preserves semantic obligations that remain after validated
    Sec Semantic IR enters MLIR. Schema version 3 completes active scalar
    coverage, carries explicit target-plan metadata, and represents checked
    builtin integer semantics while retaining earlier schema constructs.
  }];
  let cppNamespace = "::sec";
  let useDefaultTypePrinterParser = 1;
  let useDefaultAttributePrinterParser = 1;
  let isExtensible = 0;
  let extraClassDeclaration = [{
    static constexpr unsigned kSchemaVersion = 7;
    void registerAttributes();
    void registerTypes();
    void registerOperations();
    ::llvm::LogicalResult verifyOperationAttribute(
        ::mlir::Operation *operation,
        ::mlir::NamedAttribute attribute) override;
  }];
}

#endif // SEC_DIALECT_SEC_DIALECT_TD
