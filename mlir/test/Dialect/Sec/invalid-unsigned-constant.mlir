// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid() {
    %value = "sec.const.int"() <{value = "-1"}> : () -> ui64
    return
  }
}

// CHECK: unsigned result cannot contain a negative value
