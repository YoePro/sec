#ifndef SEC_CONVERSION_SECTOLLVM_PASSES_TD
#define SEC_CONVERSION_SECTOLLVM_PASSES_TD

include "mlir/Pass/PassBase.td"

def SecCheckLoweringSupport : Pass<"sec-check-lowering-support", "mlir::ModuleOp"> {
  let summary = "Reject Sec operations the LLVM lowering track does not lower yet";
  let description = [{
    The entry gate of the Sec MLIR -> LLVM track. Every Sec operation must be
    in the set the track lowers; any other one is reported by name before a
    partial conversion can fail with an internal materialization error. The
    set grows as packages add lowering.
  }];
}

def SecLowerIntegerEnums : Pass<"sec-lower-integer-enums", "mlir::ModuleOp"> {
  let summary = "Discharge ordinary integer enum constants and equality into their resolved representation";
  let description = [{
    After scalar layout, erase ordinary integer enum identities for constants,
    equality, storage and direct calls. Case values come from the verified
    case table, never the ordinal. Enum conversions and bit-backed enums are
    outside this slice. Earlier package pipelines retain nominal enum types.
  }];
  let dependentDialects = [
    "mlir::arith::ArithDialect",
    "mlir::func::FuncDialect"
  ];
}

def SecLowerFailureEndpoints : Pass<"sec-lower-failure-endpoints", "mlir::ModuleOp"> {
  let summary = "Lower deterministic Sec failures to the minimal panic endpoint";
  let description = [{
    Gives arithmetic failure reasons their i8 code and replaces every
    non-returning Sec failure terminator (fail.arithmetic, fail.bounds,
    fail.reference_generation, synthesized unreachable) by the minimal
    hosted panic endpoint: llvm.intr.trap followed by llvm.unreachable.
  }];
  let dependentDialects = [
    "mlir::arith::ArithDialect",
    "mlir::LLVM::LLVMDialect"
  ];
}

def SecLowerCommandEntry : Pass<"sec-lower-command-entry", "mlir::ModuleOp"> {
  let summary = "Generate the hosted startup adapter for command main";
  let description = [{
    Finds the source-level command entry `fn main() int` by its Sec function
    identity and generates the platform entry symbol `main` returning the
    termination status as the C int of the hosted target.
  }];
  let options = [
    Option<"entry", "entry", "std::string", "\"main::main()\"",
           "Sec function identity of the command entry">
  ];
  let dependentDialects = [
    "mlir::arith::ArithDialect",
    "mlir::func::FuncDialect"
  ];
}

def SecFinalizeForLLVM : Pass<"sec-finalize-for-llvm", "mlir::ModuleOp"> {
  let summary = "Reject unlowered Sec constructs and drop Sec metadata";
  let description = [{
    The single gate between Sec semantics and standard LLVM conversion: any
    remaining Sec operation or type is a hard error naming the construct the
    lowering path does not support yet. Otherwise the module receives the
    target triple and every sec.* attribute is removed.
  }];
}

#endif // SEC_CONVERSION_SECTOLLVM_PASSES_TD
