// RUN: not sec-mlir-opt %s -split-input-file -o /dev/null 2>&1 | FileCheck %s

// v10 section 10A: the value has the element type and the result is an array.

// CHECK: 'sec.array.fill' op value type must match the result element type
module {
  func.func @mistyped(%seed: i64) -> !sec.array<i128, "4"> {
    %value = "sec.array.fill"(%seed) : (i64) -> !sec.array<i128, "4">
    return %value : !sec.array<i128, "4">
  }
}

// -----

// CHECK: 'sec.array.fill' op result must have !sec.array type
module {
  func.func @scalar(%seed: i128) -> i128 {
    %value = "sec.array.fill"(%seed) : (i128) -> i128
    return %value : i128
  }
}
