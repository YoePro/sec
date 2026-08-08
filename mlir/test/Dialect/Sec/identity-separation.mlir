// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @different_identities(
      %value: !sec.named<"A", i64>) -> !sec.named<"B", i64> {
    return %value : !sec.named<"A", i64>
  }
}

// CHECK: type of return operand 0 ('!sec.named<"A", i64>') doesn't match function result type ('!sec.named<"B", i64>')
