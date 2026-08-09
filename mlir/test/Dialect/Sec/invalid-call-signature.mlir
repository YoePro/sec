// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func private @target(i32) -> i32 attributes {sec.extern = false}
  func.func @invalid(%value: i64) {
    %result = "sec.call.direct"(%value) <{callee = @target}> {sec.argument_actions = ["copy-trivial"]} : (i64) -> i32
    return
  }
}

// CHECK: operand types must match callee inputs
