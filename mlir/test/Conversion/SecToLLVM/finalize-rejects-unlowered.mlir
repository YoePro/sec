// RUN: sec-mlir-opt %s --sec-finalize-for-llvm --split-input-file --verify-diagnostics

// The gate before standard LLVM conversion names every Sec construct the
// basic path does not lower yet instead of guessing a representation.

module {
  func.func @text() {
    // expected-error @+1 {{the Sec MLIR -> LLVM track does not lower 'sec.const.string' yet}}
    %s = "sec.const.string"() <{value = "hi"}> : () -> !sec.string
    return
  }
}

// -----

module {
  // expected-error @+1 {{the Sec MLIR -> LLVM track does not lower values of type !sec.int yet}}
  func.func @unlowered(%value: !sec.int) -> !sec.int {
    return %value : !sec.int
  }
}
