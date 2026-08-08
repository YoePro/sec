// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%value: !sec.named<"main::Invalid", none>)
}

// CHECK: sec.named base type must not be none
