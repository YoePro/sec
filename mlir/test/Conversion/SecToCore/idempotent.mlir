// RUN: sec-mlir-opt %s --sec-lower-trivial-core --sec-lower-trivial-core | FileCheck %s

module {
  func.func @idempotent() -> i1 attributes {sec.extern = false} {
    %value = "sec.const.bool"() <{value = false}> : () -> i1
    return %value : i1
  }
}

// CHECK-COUNT-1: arith.constant false
// CHECK-NOT: sec.const.bool
// CHECK-NOT: unrealized_conversion_cast
