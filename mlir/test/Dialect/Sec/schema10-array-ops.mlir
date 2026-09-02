// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module {
  func.func @default() -> !sec.array<i128, "4"> {
    %value = "sec.array.default"() : () -> !sec.array<i128, "4">
    return %value : !sec.array<i128, "4">
  }

  func.func @length(%array: !sec.array<i128, "4">) -> !sec.uint {
    %length = "sec.array.len"(%array) : (!sec.array<i128, "4">) -> !sec.uint
    return %length : !sec.uint
  }

  func.func @proven(%array: !sec.array<i128, "4">, %index: ui128, %new_value: i128) -> !sec.array<i128, "4"> {
    %inside = "sec.array.index_in_bounds"(%array, %index) <{index_signed = false}> : (!sec.array<i128, "4">, ui128) -> i1
    %value = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<i128, "4">, ui128) -> i128
    %updated = "sec.array.replace"(%array, %index, %new_value) <{bounds_kind = "proven-safe", bounds_proof = "contract"}> : (!sec.array<i128, "4">, ui128, i128) -> !sec.array<i128, "4">
    return %updated : !sec.array<i128, "4">
  }

  func.func @runtime(%array: !sec.array<i128, "4">, %index: si128) -> i128 {
    %inside = "sec.array.index_in_bounds"(%array, %index) <{index_signed = true}> : (!sec.array<i128, "4">, si128) -> i1
    cf.cond_br %inside, ^success, ^failure
  ^success:
    %value = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i128, "4">, si128) -> i128
    return %value : i128
  ^failure:
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }
}

// CHECK: "sec.array.default"() : () -> !sec.array<i128, "4">
// CHECK: "sec.array.len"(%{{.*}}) : (!sec.array<i128, "4">) -> !sec.uint
// CHECK: "sec.array.index_in_bounds"(%{{.*}}, %{{.*}}) <{index_signed = false}>
// CHECK: "sec.array.extract"(%{{.*}}, %{{.*}}) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "range"}>
// CHECK: "sec.array.replace"(%{{.*}}, %{{.*}}, %{{.*}}) <{bounds_kind = "proven-safe", bounds_proof = "contract"}>
// CHECK: "sec.array.index_in_bounds"(%{{.*}}, %{{.*}}) <{index_signed = true}>
// CHECK: bounds_kind = "runtime-check", bounds_proof = "guarded"
// CHECK: "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
