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

def SecAnnotateSuccessorActions : Pass<"sec-annotate-successor-actions", "mlir::ModuleOp"> {
  let summary = "Restore branch ownership action metadata dropped by optimization";
  let description = [{
    `sec.successor_actions` is dialect-scoped discardable metadata on
    `cf.br` / `cf.cond_br` (dialect v13 section 26), so a rewrite that
    recreates a branch drops it. The action of a forwarded operand is fixed by
    its type: "move" for a value that needs destruction, "non-owning" for a
    reference or slice holder, "copy" otherwise. This pass writes the missing
    metadata of every ownership-sensitive branch on schema 13 and leaves
    existing metadata for --sec-verify-ownership to check. An optimization
    pipeline over schema 13 runs it before ownership verification (P17-57).
  }];
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
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
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 41, 44-56, 70-72, 79, 81.
def SecVerifyNumericConversions : Pass<"sec-verify-numeric-conversions", "mlir::ModuleOp"> {
  let summary = "Verify exact-versus-checked classification, success-only candidates and ordered ConversionError reasons";
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

def SecVerifyFloatSemantics : Pass<"sec-verify-float-semantics", "mlir::ModuleOp"> {
  let summary = "Verify strict Sec binary floating semantics and Clamp guards";
  let dependentDialects = ["mlir::arith::ArithDialect", "mlir::cf::ControlFlowDialect"];
}

// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 37, 63–67, 128–130.
def SecVerifyDynamicArrays : Pass<"sec-verify-dynamic-arrays", "mlir::func::FuncOp"> {
  let summary = "Verify the dynamic-array growth and storage-transition protocol";
  let description = [{
    A grow_backing or allocate is immediately tested on failed; an allocate
    candidate never reaches its failure edge; the matching establish_domain or
    advance_epoch opens the success edge of its grow_backing, which follows the
    failed has_backing (establish) or has_capacity (advance) test of the same
    owner Place; append_prepared opens a block entered only after capacity is
    ensured for the same owner Place.
  }];
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 124, 126-127, 129.
def SecVerifyArenas : Pass<"sec-verify-arenas", "mlir::func::FuncOp"> {
  let summary = "Verify Arena domain, owner and epoch flow (schema 15)";
  let description = [{
    Every creation establishes a fresh domain; an Arena owner is consumed by
    at most one Release/destroy on any path and never used after it; state
    versions and epochs of one domain chain within a block; a reference or
    slice from arena.new/alloc is not used after a Reset that advanced past
    its epoch or after its domain ended. Sema remains the liveness authority;
    this checks the emitted facts.
  }];
}

// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 107-109, 125, 128.
def SecVerifyAllocationContexts : Pass<"sec-verify-allocation-contexts", "mlir::ModuleOp"> {
  let summary = "Verify hidden allocation contexts and their propagation (schema 15)";
  let description = [{
    A !sec.alloc_context argument is the first argument, marked
    sec.hidden_allocation_context, unique, and absent from foreign/extern
    functions; a direct call to a function with a hidden context forwards one
    as its first operand; contexts are used only by owned Arena creation and
    such calls, never as block arguments, results or other operands.
  }];
}

// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 30–31, 37, 58–67, 129–130.
def SecVerifyStorageTransitions : Pass<"sec-verify-storage-transitions", "mlir::func::FuncOp"> {
  let summary = "Verify storage-domain lifecycle and dynamic-array growth invariants";
  let description = [{
    Within a function, one owner names one backing domain; nothing depends on
    an owner after its storage.end_domain; storage.reclaim directly follows the
    end_domain or advance_epoch of the same owner and domain; advance_epoch
    grows only an owner proven to have backing; no element Place or slice of
    the owner lives across its grow_backing; a failed growth applies no
    transition; and establishment facts of one domain agree.
  }];
  let dependentDialects = ["mlir::cf::ControlFlowDialect"];
}

#endif // SEC_ANALYSIS_PASSES_TD
