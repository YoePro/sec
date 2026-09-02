// RUN: sec-mlir-opt %s | FileCheck %s

module attributes {
  sec.dialect_version = 10 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema10",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
}

// CHECK-DAG: sec.dialect_version = 10 : i32
// CHECK-DAG: sec.semantic_ir_version = 1 : i32
// CHECK-DAG: sec.module_id = "schema10"
