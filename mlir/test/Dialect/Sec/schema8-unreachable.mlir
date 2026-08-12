// RUN: sec-mlir-opt %s | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect = "sec",
  sec.dialect_version = 8 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema8-unreachable",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @impossible() {
    "sec.unreachable"() <{reason = "exhaustive-match-fallthrough"}> {sec.synthesized = true} : () -> ()
  }
}

// CHECK: sec.dialect_version = 8 : i32
// CHECK: "sec.unreachable"() <{reason = "exhaustive-match-fallthrough"}> {sec.synthesized = true}
