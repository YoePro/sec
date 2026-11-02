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

def SecVerifyCleanups : Pass<"sec-verify-cleanups", "mlir::func::FuncOp"> {
  let summary = "Verify cleanup registration, cancellation and exits path-sensitively";
  let description = [{
    Follows each cleanup registration over the CFG: cancellation of an active
    responsibility only, no cleanup of a moved value, returned values excluded,
    run_function before every return with live registrations, and none on
    non-returning paths or inside a defer body.
  }];
}

def SecVerifyOwnership : Pass<"sec-verify-ownership", "mlir::ModuleOp"> {
  let summary = "Verify owned values path-sensitively";
  let description = [{
    Follows every owned value and subobject over the CFG: copyability, availability
    and use after move, partial subobject state, initialization and replacement,
    mutually exclusive transfers and merges, and one terminal ownership action for
    every value that needs destruction. Borrow validity stays with the P15 verifiers.
  }];
  let dependentDialects = ["mlir::cf::ControlFlowDialect", "mlir::func::FuncDialect"];
}

def SecVerifyDestructionPlans : Pass<"sec-verify-destruction-plans", "mlir::ModuleOp"> {
  let summary = "Verify the type-level destruction plan table and its uses";
  let description = [{
    Checks that sec.destruction_plans is closed and complete: one plan per type,
    struct fields once in reverse declaration order, the exact array element plan,
    active-variant payload coverage and custom-free restrictions. Every operation
    naming a plan must name one of exactly the destroyed type.
  }];
  let dependentDialects = ["mlir::func::FuncDialect"];
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
