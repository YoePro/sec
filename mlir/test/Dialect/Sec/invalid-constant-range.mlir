// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid() {
    %value = "sec.const.int"() <{value = "128"}> : () -> si8
    return
  }
}

// CHECK: integer value is not representable by result type
