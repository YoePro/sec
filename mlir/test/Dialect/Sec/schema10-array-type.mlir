// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module {
  func.func @zero(%value: !sec.array<i32, "0">) -> !sec.array<i32, "0"> {
    return %value : !sec.array<i32, "0">
  }

  func.func @wide(%value: !sec.array<i128, "18446744073709551616">) -> !sec.array<i128, "18446744073709551616"> {
    return %value : !sec.array<i128, "18446744073709551616">
  }

  func.func private @identity(
      !sec.array<i32, "4">,
      !sec.array<i32, "5">,
      !sec.array<ui32, "4">)

  func.func @nested(%value: !sec.array<!sec.array<ui256, "3">, "2">) -> !sec.array<!sec.array<ui256, "3">, "2"> {
    return %value : !sec.array<!sec.array<ui256, "3">, "2">
  }

  func.func @struct_element(
      %value: !sec.array<!sec.struct<identity = "main::Item", typeArguments = [], fields = []>, "4">)
      -> !sec.array<!sec.struct<identity = "main::Item", typeArguments = [], fields = []>, "4"> {
    return %value : !sec.array<!sec.struct<identity = "main::Item", typeArguments = [], fields = []>, "4">
  }
}

// CHECK: !sec.array<i32, "0">
// CHECK: !sec.array<i128, "18446744073709551616">
// CHECK: func.func private @identity(!sec.array<i32, "4">, !sec.array<i32, "5">, !sec.array<ui32, "4">)
// CHECK: !sec.array<!sec.array<ui256, "3">, "2">
// CHECK: !sec.array<!sec.struct<identity = "main::Item", typeArguments = [], fields = []>, "4">
