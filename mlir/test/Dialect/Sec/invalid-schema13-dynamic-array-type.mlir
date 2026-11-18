// RUN: not sec-mlir-opt %s -o /dev/null 2>&1 | FileCheck %s

// SEC-MLIR-P18 section 127: schema-13 regression - a dynamic-array type in a
// schema-13 module is rejected by name.

// CHECK: dynamic-array types require schema 14, module declares schema 13
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "types", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @old(%values: !sec.dynamic_array<ui8>) {
    return
  }
}
