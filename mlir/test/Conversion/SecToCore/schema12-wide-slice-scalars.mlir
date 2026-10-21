// RUN: split-file %s %t
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout --sec-lower-checked-integers | FileCheck %s
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout --sec-lower-checked-integers | FileCheck %s

// P16-49 and P16 sections 3/112-113: fixed-width payloads stay semantic
// under P6/P8 and do not inherit the target-sized length representation.
// CHECK-LABEL: func.func @wide(
// CHECK-SAME: !sec.slice<si128>
// CHECK-SAME: !sec.slice_mut<si256>
// CHECK-SAME: !sec.slice<ui128>
// CHECK-SAME: !sec.slice_mut<ui256>
// CHECK-SAME: !sec.slice<!sec.decimal128>
// CHECK-SAME: -> !sec.slice<!sec.decimal128>

//--- 32.mlir
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "wide-slice-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @wide(%signed128: !sec.slice<si128>, %signed256: !sec.slice_mut<si256>, %unsigned128: !sec.slice<ui128>, %unsigned256: !sec.slice_mut<ui256>, %decimal: !sec.slice<!sec.decimal128>) -> !sec.slice<!sec.decimal128> {
    return %decimal : !sec.slice<!sec.decimal128>
  }
}

//--- 64.mlir
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "wide-slice-64", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wide(%signed128: !sec.slice<si128>, %signed256: !sec.slice_mut<si256>, %unsigned128: !sec.slice<ui128>, %unsigned256: !sec.slice_mut<ui256>, %decimal: !sec.slice<!sec.decimal128>) -> !sec.slice<!sec.decimal128> {
    return %decimal : !sec.slice<!sec.decimal128>
  }
}
