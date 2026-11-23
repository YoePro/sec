// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// SEC-MLIR-P20 sections 73-74: schema 16 admits every earlier operation and
// requires the canonical bits of each float constant; the lexeme is only
// provenance.

module attributes {sec.dialect_version = 16 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @canonical() -> f64 {
    %v = "sec.const.float"() <{bits = 4602678819172646912 : i64, lexeme = "0.5"}> : () -> f64
    return %v : f64
  }
}

// -----

module attributes {sec.dialect_version = 16 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @lexeme_only() -> f64 {
    // expected-error@+1 {{schema 16 requires the canonical bits of every float constant}}
    %v = "sec.const.float"() <{lexeme = "0.5"}> : () -> f64
    return %v : f64
  }
}
