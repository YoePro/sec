#ifndef SEC_CONVERSION_SECTOCORE_PASSES_TD
#define SEC_CONVERSION_SECTOCORE_PASSES_TD

include "mlir/Pass/PassBase.td"

def SecLowerTrivialCore : Pass<"sec-lower-trivial-core", "mlir::ModuleOp"> {
  let summary = "Lower fully discharged Sec scalar semantics to core MLIR";
  let description = [{
    Partially lowers boolean constants, trivial scalar automatic storage, and
    resolved direct calls. Sec constructs with unresolved representation or ABI
    obligations remain explicit.
  }];
  let dependentDialects = [
    "mlir::arith::ArithDialect",
    "mlir::func::FuncDialect",
    "mlir::memref::MemRefDialect"
  ];
}

def SecResolveScalarLayout : Pass<"sec-resolve-scalar-layout", "mlir::ModuleOp"> {
  let summary = "Resolve Sec scalar types from explicit target data layout";
  let description = [{
    Resolves target-sized and representation-fixed scalar types while retaining
    signedness, nominal identity, foreign-call identity, and unresolved decimal
    and string semantics.
  }];
  let dependentDialects = [
    "mlir::arith::ArithDialect",
    "mlir::cf::ControlFlowDialect",
    "mlir::func::FuncDialect",
    "mlir::memref::MemRefDialect"
  ];
}

#endif // SEC_CONVERSION_SECTOCORE_PASSES_TD
