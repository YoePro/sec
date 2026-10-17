// RUN: sec-mlir-opt %s --sec-check-lowering-support --verify-diagnostics

// The entry gate names every Sec operation outside the set the track lowers
// before a partial conversion could fail with an internal materialization
// error; supported operations pass silently.

module {
  func.func @mixed(%flag: i1) -> i1 {
    %b = "sec.const.bool"() <{value = true}> : () -> i1
    // expected-error @+1 {{the Sec MLIR -> LLVM track does not lower 'sec.const.string' yet}}
    %s = "sec.const.string"() <{value = "hi"}> : () -> !sec.string
    return %b : i1
  }
}
