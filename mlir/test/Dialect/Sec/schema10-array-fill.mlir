// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// compiler_known_members.md "Contextual fill", v10 section 10A: fill(value)
// copies one value into every element of a fixed array, including none.

// CHECK-LABEL: func.func @fill
// CHECK: "sec.array.fill"(%{{.*}}) : (i128) -> !sec.array<i128, "4">
// CHECK-LABEL: func.func @empty
// CHECK: "sec.array.fill"(%{{.*}}) : (i128) -> !sec.array<i128, "0">
module {
  func.func @fill(%seed: i128) -> !sec.array<i128, "4"> {
    %value = "sec.array.fill"(%seed) : (i128) -> !sec.array<i128, "4">
    return %value : !sec.array<i128, "4">
  }

  func.func @empty(%seed: i128) -> !sec.array<i128, "0"> {
    %value = "sec.array.fill"(%seed) : (i128) -> !sec.array<i128, "0">
    return %value : !sec.array<i128, "0">
  }
}
