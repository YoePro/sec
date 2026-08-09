// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%value: !sec.storage<!sec.storage<i32>>)
}

// CHECK: sec.storage cannot contain another sec.storage
