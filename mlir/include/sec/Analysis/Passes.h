#ifndef SEC_ANALYSIS_PASSES_H
#define SEC_ANALYSIS_PASSES_H

#include "mlir/Pass/Pass.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"

namespace sec {

#define GEN_PASS_DECL
#include "sec/Analysis/Passes.h.inc"

std::unique_ptr<mlir::Pass> createSecVerifyCheckedIntegerGuardsPass();
mlir::LogicalResult verifyCheckedIntegerGuards(mlir::func::FuncOp function);
std::unique_ptr<mlir::Pass> createSecVerifyResultGuardsPass();
mlir::LogicalResult verifyResultGuards(mlir::func::FuncOp function);
std::unique_ptr<mlir::Pass> createSecVerifyTryHandlersPass();
mlir::LogicalResult verifyTryHandlers(mlir::func::FuncOp function);
std::unique_ptr<mlir::Pass> createSecVerifyUnionGuardsPass();
mlir::LogicalResult verifyUnionGuards(mlir::func::FuncOp function);
std::unique_ptr<mlir::Pass> createSecVerifyMatchCFGPass();
mlir::LogicalResult verifyMatchCFG(mlir::func::FuncOp function);

#define GEN_PASS_REGISTRATION
#include "sec/Analysis/Passes.h.inc"

} // namespace sec

#endif // SEC_ANALYSIS_PASSES_H
