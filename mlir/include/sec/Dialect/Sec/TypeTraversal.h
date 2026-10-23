#ifndef SEC_DIALECT_SEC_TYPETRAVERSAL_H
#define SEC_DIALECT_SEC_TYPETRAVERSAL_H

#include "mlir/IR/Operation.h"
#include "llvm/ADT/STLFunctionalExtras.h"

namespace sec {

// Find a nested type in this operation's attributes, SSA values or region
// arguments. The caller owns operation traversal; this does not visit child
// operations. Signatures and aggregate field attributes are inspected too.
// Rules: rules/mlir/sec_mlir_dialect.md, v11.41.1 schema admission;
// rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 1-5;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md sections 1-4.
inline mlir::Type findOperationType(
    mlir::Operation *operation, llvm::function_ref<bool(mlir::Type)> matches) {
  mlir::Type found;
  auto checkType = [&](mlir::Type type) {
    type.walk([&](mlir::Type nested) {
      if (!found && matches(nested)) found = nested;
    });
  };
  operation->getAttrDictionary().walk(checkType);
  for (mlir::Type type : operation->getOperandTypes()) checkType(type);
  for (mlir::Type type : operation->getResultTypes()) checkType(type);
  for (mlir::Region &region : operation->getRegions())
    for (mlir::Block &block : region)
      for (mlir::BlockArgument argument : block.getArguments())
        checkType(argument.getType());
  return found;
}

} // namespace sec

#endif // SEC_DIALECT_SEC_TYPETRAVERSAL_H
