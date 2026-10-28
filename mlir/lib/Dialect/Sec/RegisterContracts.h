#ifndef SEC_REGISTER_CONTRACTS_H
#define SEC_REGISTER_CONTRACTS_H
#include "sec/Dialect/Sec/SecAttributes.h"
#include "mlir/IR/BuiltinOps.h"
namespace sec::detail {
// The single MD-114 module registry validates nominal definitions and paths.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §§2–9).
mlir::LogicalResult verifyRegisterModule(mlir::ModuleOp module);
RegisterDefAttr lookupRegister(mlir::Operation *anchor, mlir::Type type);
mlir::LogicalResult verifyRegisterWrite(mlir::Operation *operation, RegisterFieldAttr field);
mlir::FailureOr<RegisterFieldAttr> lookupRegisterField(mlir::Operation *operation);
}
#endif
