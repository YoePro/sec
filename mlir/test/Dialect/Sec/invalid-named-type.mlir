// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%value: !sec.named<"", i64>)
}

// CHECK: sec.named identity must not be empty
