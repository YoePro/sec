// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func private @foreign(i32) attributes {sec.extern = true}
  func.func @invalid(%value: i32) attributes {sec.extern = false} {
    "sec.call.direct"(%value) <{callee = @foreign}> {sec.argument_actions = ["copy-trivial"]} : (i32) -> ()
    return
  }
}

// CHECK: direct call target must not be extern
