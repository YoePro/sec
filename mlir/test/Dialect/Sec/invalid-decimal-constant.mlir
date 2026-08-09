// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid() {
    %value = "sec.const.decimal"() <{coefficient = "not-an-integer", lexeme = "1.0", scale = 1 : i32}> : () -> !sec.decimal
    return
  }
}

// CHECK: coefficient must be a base-10 integer string
