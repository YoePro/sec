// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 3 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema3",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @scalars(%i128: si128, %i256: si256, %u128: ui128, %u256: ui256, %decimal: !sec.decimal128) attributes {sec.extern = false} {
    return
  }
}

// CHECK: dlti.dl_spec = #dlti.dl_spec<index = 64 : i64>
// CHECK: sec.dialect_version = 3 : i32
// CHECK: sec.target_arch = "amd64"
// CHECK: si128
// CHECK: si256
// CHECK: ui128
// CHECK: ui256
// CHECK: !sec.decimal128
