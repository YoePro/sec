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

def SecVerifyMatchCFG : Pass<"sec-verify-match-cfg", "mlir::func::FuncOp"> {
  let summary = "Verify source-ordered Sec match CFG and provenance";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

def SecVerifyArrayIndexGuards : Pass<"sec-verify-array-index-guards", "mlir::func::FuncOp"> {
  let summary = "Verify canonical Sec fixed-array index guards";
  let description = [{
    Verifies that every runtime-checked fixed-array extract or replacement is
    dominated by the true edge of a matching sec.array.index_in_bounds test on
    the same array and index SSA values. Proven-safe operations must instead
    carry compiler-owned proof provenance.
  }];
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

// Rules: rules/mlir/sec_mlir_dialect.md — v12.24.
def SecVerifySliceGuards : Pass<"sec-verify-slice-guards", "mlir::func::FuncOp"> {
  let summary = "Verify Sec slice range/index and temporal guard control flow";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

// Schema 11 (SEC-MLIR-P15). Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md
// sections 34-37.
def SecVerifyPlaces : Pass<"sec-verify-places", "mlir::func::FuncOp"> {
  let summary = "Verify Sec Place non-escape, element bounds guards and payload variant guards";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

def SecVerifyReferenceGuards : Pass<"sec-verify-reference-guards", "mlir::func::FuncOp"> {
  let summary = "Verify validity guards of dynamic-epoch Sec reference uses";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

def SecVerifyBorrowSemantics : Pass<"sec-verify-borrow-semantics", "mlir::func::FuncOp"> {
  let summary = "Verify Sec borrow derivation, end-borrow ordering and reference moves";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§6–9, 81.
def SecVerifyFloatSemantics : Pass<"sec-verify-float-semantics", "mlir::ModuleOp"> {
  let summary = "Verify strict Sec binary floating semantics and Clamp guards";
  let dependentDialects = ["mlir::arith::ArithDialect", "mlir::cf::ControlFlowDialect"];
}

#endif // SEC_ANALYSIS_PASSES_TD
