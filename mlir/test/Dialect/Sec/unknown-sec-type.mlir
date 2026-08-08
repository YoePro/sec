// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%value: !sec.future_type)
}

// CHECK: unknown  type `future_type` in dialect `sec`
