// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%value: ui256) {
    %result, %failed = "sec.int.neg_checked"(%value) : (ui256) -> (ui256, i1)
    return
  }
}

// CHECK: operand must be a signed builtin Sec integer semantic type
