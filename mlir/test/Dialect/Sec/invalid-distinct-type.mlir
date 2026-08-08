// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%value: !sec.distinct<"", i64>)
}

// CHECK: sec.distinct identity must not be empty
