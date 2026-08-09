// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func private @target(i32) attributes {sec.extern = false}
  func.func @invalid(%value: i32) {
    "sec.call.direct"(%value) <{callee = @target}> {sec.argument_actions = ["move"]} : (i32) -> ()
    return
  }
}

// CHECK: only copy-trivial argument actions are valid in schema 2
