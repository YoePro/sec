// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module {
  func.func @empty() -> !sec.array<i32, "0"> {
    %value = "sec.array.construct"() <{segment_actions = [], segment_kinds = [], segment_lengths = []}> : () -> !sec.array<i32, "0">
    return %value : !sec.array<i32, "0">
  }

  func.func @segments(%first: i32, %middle: !sec.array<i32, "3">, %last: i32) -> !sec.array<i32, "5"> {
    %value = "sec.array.construct"(%first, %middle, %last) <{segment_actions = ["construct-direct", "copy-trivial", "construct-direct"], segment_kinds = ["element", "spread", "element"], segment_lengths = ["1", "3", "1"]}> : (i32, !sec.array<i32, "3">, i32) -> !sec.array<i32, "5">
    return %value : !sec.array<i32, "5">
  }

  func.func @huge(%spread: !sec.array<i128, "18446744073709551616">) -> !sec.array<i128, "18446744073709551616"> {
    %value = "sec.array.construct"(%spread) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["18446744073709551616"]}> : (!sec.array<i128, "18446744073709551616">) -> !sec.array<i128, "18446744073709551616">
    return %value : !sec.array<i128, "18446744073709551616">
  }
}

// CHECK: "sec.array.construct"() <{segment_actions = [], segment_kinds = [], segment_lengths = []}>
// CHECK: "sec.array.construct"(%{{.*}}, %{{.*}}, %{{.*}}) <{segment_actions = ["construct-direct", "copy-trivial", "construct-direct"], segment_kinds = ["element", "spread", "element"], segment_lengths = ["1", "3", "1"]}>
// CHECK: segment_lengths = ["18446744073709551616"]
