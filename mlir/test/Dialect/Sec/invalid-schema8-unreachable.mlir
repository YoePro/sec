// RUN: split-file %s %t
// RUN: not sec-mlir-opt %t/not-synthesized.mlir 2>&1 | FileCheck %s --check-prefix=SYNTHESIZED
// RUN: not sec-mlir-opt %t/empty-reason.mlir 2>&1 | FileCheck %s --check-prefix=REASON

//--- not-synthesized.mlir
module {
  func.func @invalid() {
    "sec.unreachable"() <{reason = "exhaustive-match-fallthrough"}> : () -> ()
  }
}
// SYNTHESIZED: requires sec.synthesized = true

//--- empty-reason.mlir
module {
  func.func @invalid() {
    "sec.unreachable"() <{reason = ""}> {sec.synthesized = true} : () -> ()
  }
}
// REASON: requires a non-empty reason
