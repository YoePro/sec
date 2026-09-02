// RUN: sec-mlir-opt --sec-lower-checked-integers %s | FileCheck %s

module {
  func.func @checked_scalar(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : si32
  }

  func.func @empty() -> !sec.array<si32, "0"> {
    %value = "sec.array.construct"() <{segment_actions = [], segment_kinds = [], segment_lengths = []}> : () -> !sec.array<si32, "0">
    return %value : !sec.array<si32, "0">
  }

  func.func @spread(%source: !sec.array<ui64, "2">) -> !sec.array<ui64, "2"> {
    %value = "sec.array.construct"(%source) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["2"]}> : (!sec.array<ui64, "2">) -> !sec.array<ui64, "2">
    return %value : !sec.array<ui64, "2">
  }

  func.func @nested(%value: !sec.array<!sec.array<si128, "3">, "4">) -> !sec.array<!sec.array<si128, "3">, "4"> {
    return %value : !sec.array<!sec.array<si128, "3">, "4">
  }
}

// CHECK-LABEL: func.func @checked_scalar(%{{.*}}: i32, %{{.*}}: i32) -> i32
// CHECK: arith.addi
// CHECK-NOT: "sec.int.binary_checked"
// CHECK-LABEL: func.func @empty() -> !sec.array<si32, "0">
// CHECK: "sec.array.construct"() <{segment_actions = [], segment_kinds = [], segment_lengths = []}> : () -> !sec.array<si32, "0">
// CHECK-LABEL: func.func @spread(%{{.*}}: !sec.array<ui64, "2">) -> !sec.array<ui64, "2">
// CHECK: "sec.array.construct"(%{{.*}}) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["2"]}> : (!sec.array<ui64, "2">) -> !sec.array<ui64, "2">
// CHECK-LABEL: func.func @nested(%{{.*}}: !sec.array<!sec.array<si128, "3">, "4">) -> !sec.array<!sec.array<si128, "3">, "4">
// CHECK-NOT: !sec.array<i32
// CHECK-NOT: !sec.array<i64
// CHECK-NOT: !sec.array<!sec.array<i128
// CHECK-NOT: unrealized_conversion_cast
// CHECK-NOT: llvm.
