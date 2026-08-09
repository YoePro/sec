#ifndef SEC_CONVERSION_SECINTEGERTOARITH_PASSES_TD
#define SEC_CONVERSION_SECINTEGERTOARITH_PASSES_TD

include "mlir/Pass/PassBase.td"

def SecLowerCheckedIntegers : Pass<"sec-lower-checked-integers", "mlir::ModuleOp"> {
  let summary = "Lower total checked Sec integer semantics to signless Arith";
  let description = [{
    Verifies schema-v4 checked guards, normalizes plain builtin integer
    representation to signless integers, and lowers every sec.int operation to
    total standard Arith computations while preserving explicit failure CFG.
  }];
  let dependentDialects = [
    "mlir::arith::ArithDialect",
    "mlir::cf::ControlFlowDialect",
    "mlir::func::FuncDialect",
    "mlir::memref::MemRefDialect"
  ];
}

#endif // SEC_CONVERSION_SECINTEGERTOARITH_PASSES_TD
