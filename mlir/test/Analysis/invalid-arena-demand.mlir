// RUN: not sec-mlir-opt --split-input-file --sec-verify-arenas %s 2>&1 | FileCheck %s

// P19 § 130: sec.arena_demand_summary is compile-time metadata with one
// entry per Arena domain the function creates; constant demand carries
// decimal bytes, unknown and unbounded demand none (§ 81).

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wrong_kind() attributes {sec.arena_demand_summary = [{domain = 1 : i32, expression = "constant(8)", kind = "bounded"}]} {
    return
  }
}
// CHECK: an Arena demand summary needs a domain, kind constant|unknown|unbounded and an expression

// -----

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unknown_bytes() attributes {sec.arena_demand_summary = [{bytes = "8", domain = 1 : i32, expression = "unknown(loop)", kind = "unknown"}]} {
    return
  }
}
// CHECK: constant Arena demand carries decimal bytes; unknown or unbounded demand carries none

// -----

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unbounded_bytes() attributes {sec.arena_demand_summary = [{bytes = "8", domain = 1 : i32, expression = "unbounded(recursion through F)", kind = "unbounded"}]} {
    return
  }
}
// CHECK: constant Arena demand carries decimal bytes; unknown or unbounded demand carries none

// -----

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @not_created() attributes {sec.arena_demand_summary = [{bytes = "8", domain = 7 : i32, expression = "constant(8)", kind = "constant"}]} {
    return
  }
}
// CHECK: summarizes an Arena domain it does not create
