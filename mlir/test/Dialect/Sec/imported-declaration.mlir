// RUN: sec-mlir-opt %s | FileCheck %s

// A Sec function defined by another module is declared with sec.imported:
// private, without a body and not extern; a direct call may target it.
module {
  func.func private @sec_sym_twice(i64) -> i64 attributes {sec.function_id = "util::Twice(int)", sec.source_name = "util.Twice", sec.extern = false, sec.unsafe = false, sec.imported = true}
  func.func @caller(%value: i64) -> i64 attributes {sec.function_id = "main::Caller(int)", sec.source_name = "Caller", sec.extern = false, sec.unsafe = false} {
    %result = "sec.call.direct"(%value) <{callee = @sec_sym_twice}> {sec.argument_actions = ["copy-trivial"]} : (i64) -> i64
    return %result : i64
  }
}

// CHECK: func.func private @sec_sym_twice
// CHECK-SAME: sec.imported = true
// CHECK: sec.call.direct
