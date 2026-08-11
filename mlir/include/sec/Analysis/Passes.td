#ifndef SEC_ANALYSIS_PASSES_TD
#define SEC_ANALYSIS_PASSES_TD

include "mlir/Pass/PassBase.td"

def SecVerifyCheckedIntegerGuards : Pass<"sec-verify-checked-integer-guards", "mlir::func::FuncOp"> {
  let summary = "Verify canonical Sec checked-integer failure guards";
  let description = [{
    Verifies that every schema-v4 checked integer operation is immediately
    guarded by its failure result and branches to a dedicated matching
    sec.fail.arithmetic endpoint.
  }];
  let dependentDialects = [
    "mlir::cf::ControlFlowDialect"
  ];
}

def SecVerifyResultGuards : Pass<"sec-verify-result-guards", "mlir::func::FuncOp"> {
  let summary = "Verify canonical Sec Result test and projection guards";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

def SecVerifyTryHandlers : Pass<"sec-verify-try-handlers", "mlir::func::FuncOp"> {
  let summary = "Verify ordered and exhaustive local Sec try handlers";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

def SecVerifyUnionGuards : Pass<"sec-verify-union-guards", "mlir::func::FuncOp"> {
  let summary = "Verify canonical Sec union variant projection guards";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

#endif // SEC_ANALYSIS_PASSES_TD
