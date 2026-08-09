// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%left: si32, %right: ui32) {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, ui32) -> (si32, i1)
    return
  }
}

// CHECK: operand and result types must match exactly
