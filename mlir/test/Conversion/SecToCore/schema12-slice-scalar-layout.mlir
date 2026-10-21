// RUN: split-file %s %t
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout | FileCheck %s --check-prefix=P32
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout --sec-lower-checked-integers | FileCheck %s --check-prefix=N32
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout | FileCheck %s --check-prefix=P64
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout --sec-lower-checked-integers | FileCheck %s --check-prefix=N64

// P16-46: P6 resolves scalar leaves through shared/mutable, nominal, array
// and nested slice wrappers. P8 retains semantic element signedness and
// existing borrow-end facts; neither pass chooses a physical slice layout.
// P32-LABEL: func.func @shared(
// P32-SAME: !sec.slice<si32>
// P32-LABEL: func.func @mutable(
// P32-SAME: !sec.slice_mut<ui32>
// P32-LABEL: func.func @nested(
// P32-SAME: !sec.slice<!sec.slice_mut<!sec.named<"main::Count", ui32>>>
// P32-LABEL: func.func @array(
// P32-SAME: !sec.slice<!sec.array<si32, "18446744073709551617">>
// P32-LABEL: func.func @width(
// P32-SAME: -> ui32
// P32-LABEL: func.func @marker(
// P32-SAME: !sec.slice<si32>
// P32: borrow_id = 7 : i32
// P32: "sec.ref.end_borrow"
// P32-SAME: borrow_id = 7 : i32
// P32-SAME: lifetime_id = 9 : i32
// P32-SAME: (!sec.slice<si32>) -> ()
// N32-LABEL: func.func @shared(
// N32-SAME: !sec.slice<si32>
// N32-LABEL: func.func @mutable(
// N32-SAME: !sec.slice_mut<ui32>
// N32-LABEL: func.func @nested(
// N32-SAME: !sec.slice<!sec.slice_mut<!sec.named<"main::Count", ui32>>>
// N32-LABEL: func.func @array(
// N32-SAME: !sec.slice<!sec.array<si32, "18446744073709551617">>
// N32-LABEL: func.func @width(
// N32-SAME: -> i32
// N32-LABEL: func.func @marker(
// N32-SAME: !sec.slice<si32>
// N32: borrow_id = 7 : i32
// N32: "sec.ref.end_borrow"
// N32-SAME: borrow_id = 7 : i32
// N32-SAME: lifetime_id = 9 : i32
// N32-SAME: (!sec.slice<si32>) -> ()
// P64-LABEL: func.func @shared(
// P64-SAME: !sec.slice<si64>
// P64-LABEL: func.func @mutable(
// P64-SAME: !sec.slice_mut<ui64>
// P64-LABEL: func.func @nested(
// P64-SAME: !sec.slice<!sec.slice_mut<!sec.named<"main::Count", ui64>>>
// P64-LABEL: func.func @array(
// P64-SAME: !sec.slice<!sec.array<si64, "18446744073709551617">>
// P64-LABEL: func.func @width(
// P64-SAME: -> ui64
// P64-LABEL: func.func @marker(
// P64-SAME: !sec.slice<si64>
// P64: borrow_id = 7 : i32
// P64: "sec.ref.end_borrow"
// P64-SAME: borrow_id = 7 : i32
// P64-SAME: lifetime_id = 9 : i32
// P64-SAME: (!sec.slice<si64>) -> ()
// N64-LABEL: func.func @shared(
// N64-SAME: !sec.slice<si64>
// N64-LABEL: func.func @mutable(
// N64-SAME: !sec.slice_mut<ui64>
// N64-LABEL: func.func @nested(
// N64-SAME: !sec.slice<!sec.slice_mut<!sec.named<"main::Count", ui64>>>
// N64-LABEL: func.func @array(
// N64-SAME: !sec.slice<!sec.array<si64, "18446744073709551617">>
// N64-LABEL: func.func @width(
// N64-SAME: -> i64
// N64-LABEL: func.func @marker(
// N64-SAME: !sec.slice<si64>
// N64: borrow_id = 7 : i32
// N64: "sec.ref.end_borrow"
// N64-SAME: borrow_id = 7 : i32
// N64-SAME: lifetime_id = 9 : i32
// N64-SAME: (!sec.slice<si64>) -> ()

//--- 32.mlir
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-scalar-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @shared(%value: !sec.slice<!sec.int>) -> !sec.slice<!sec.int> {
    return %value : !sec.slice<!sec.int>
  }
  func.func @mutable(%value: !sec.slice_mut<!sec.uint>) -> !sec.slice_mut<!sec.uint> {
    return %value : !sec.slice_mut<!sec.uint>
  }
  func.func @nested(%value: !sec.slice<!sec.slice_mut<!sec.named<"main::Count", !sec.uint>>>) -> !sec.slice<!sec.slice_mut<!sec.named<"main::Count", !sec.uint>>> {
    return %value : !sec.slice<!sec.slice_mut<!sec.named<"main::Count", !sec.uint>>>
  }
  func.func @array(%value: !sec.slice<!sec.array<!sec.int, "18446744073709551617">>) -> !sec.slice<!sec.array<!sec.int, "18446744073709551617">> {
    return %value : !sec.slice<!sec.array<!sec.int, "18446744073709551617">>
  }
  func.func @width(%value: !sec.uint) -> !sec.uint { return %value : !sec.uint }
  func.func @marker(%value: !sec.slice<!sec.int> {sec.reference = {borrow_id = 7 : i32, lifetime_id = 9 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    "sec.ref.end_borrow"(%value) <{borrow_id = 7 : i32, lifetime_id = 9 : i32}> : (!sec.slice<!sec.int>) -> ()
    return
  }
}

//--- 64.mlir
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-scalar-64", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @shared(%value: !sec.slice<!sec.int>) -> !sec.slice<!sec.int> {
    return %value : !sec.slice<!sec.int>
  }
  func.func @mutable(%value: !sec.slice_mut<!sec.uint>) -> !sec.slice_mut<!sec.uint> {
    return %value : !sec.slice_mut<!sec.uint>
  }
  func.func @nested(%value: !sec.slice<!sec.slice_mut<!sec.named<"main::Count", !sec.uint>>>) -> !sec.slice<!sec.slice_mut<!sec.named<"main::Count", !sec.uint>>> {
    return %value : !sec.slice<!sec.slice_mut<!sec.named<"main::Count", !sec.uint>>>
  }
  func.func @array(%value: !sec.slice<!sec.array<!sec.int, "18446744073709551617">>) -> !sec.slice<!sec.array<!sec.int, "18446744073709551617">> {
    return %value : !sec.slice<!sec.array<!sec.int, "18446744073709551617">>
  }
  func.func @width(%value: !sec.uint) -> !sec.uint { return %value : !sec.uint }
  func.func @marker(%value: !sec.slice<!sec.int> {sec.reference = {borrow_id = 7 : i32, lifetime_id = 9 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    "sec.ref.end_borrow"(%value) <{borrow_id = 7 : i32, lifetime_id = 9 : i32}> : (!sec.slice<!sec.int>) -> ()
    return
  }
}
