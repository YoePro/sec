// RUN: not sec-mlir-opt %s -split-input-file -o /dev/null 2>&1 | FileCheck %s

// SEC-MLIR-P20 sections 74 and 76: a float static is its canonical bit
// pattern within its binary format, never a lexeme or an oversized payload.

// CHECK: float static requires its canonical bit pattern
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "0.5", is_mutable = false, sym_name = "half", type = f64}> {sec.source_name = "Half", sec.static_id = 1 : i32} : () -> ()
}

// -----

// CHECK: float static requires its canonical bit pattern
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "bits:4294967296", is_mutable = false, sym_name = "wide", type = f32}> {sec.source_name = "Wide", sec.static_id = 1 : i32} : () -> ()
}
