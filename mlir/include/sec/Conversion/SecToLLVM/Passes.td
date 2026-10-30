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

def SecLowerTaggedValues : Pass<"sec-lower-tagged-values", "mlir::ModuleOp"> {
  let summary = "Select the native struct representation of scalar-payload Result and union values";
  let description = [{
    After checked-integer lowering, replace semantic Result and union values
    whose payloads are resolved scalars (or such Results/unions) with an LLVM
    literal struct: the discriminator (i1 Err flag for Result, i32 variant
    index for a union) followed by one slot per payload position, inactive
    slots undefined. Construction, tests, guarded projections, calls,
    returns, branches and signatures are rewritten together. Strings,
    structs, arrays and references in payloads are outside this slice and are
    rejected at the track entry. Earlier package pipelines keep the nominal
    types (layout.md §§ 15-16).
  }];
  let dependentDialects = [
    "mlir::LLVM::LLVMDialect",
    "mlir::func::FuncDialect"
  ];
}

def SecLowerFloats : Pass<"sec-lower-floats", "mlir::ModuleOp"> {
  let summary = "Lower the P20 binary floating subset to standard Arith";
  let description = [{
    After scalar layout, sec.const.float becomes arith.constant from its
    canonical bit pattern, sec.float.neg/binary/cmp become arith.negf,
    addf/subf/mulf/divf and cmpf (oeq/une/olt/ole/ogt/oge) with no
    fast-math flags. Remainder uses exact integer significand reduction and CF,
    avoiding an undeclared external fmod runtime dependency. The P20 core members lower without a runtime library:
    abs and classify through bitcast and field masks, minimum/maximum to
    the NaN-propagating minimumf/maximumf, round through a guarded integer
    conversion that keeps the sign of zero, and property to exact constants.
  }];
  let dependentDialects = ["mlir::arith::ArithDialect", "mlir::cf::ControlFlowDialect"];
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

def SecLowerStaticStorage : Pass<"sec-lower-static-storage", "mlir::ModuleOp"> {
  let summary = "Lower Sec static storage to LLVM globals";
  let description = [{
    After integer lowering, each sec.static.global becomes an internal LLVM
    global with its exact constant initial value (constant when immutable),
    and sec.static.load/store become loads and stores through its address.
  }];
  let dependentDialects = ["mlir::LLVM::LLVMDialect"];
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
