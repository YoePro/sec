// RUN: not sec-mlir-opt %s -split-input-file --sec-verify-arenas -o /dev/null 2>&1 | FileCheck %s

// MD-136 (P19 § 166): default Arena verifier negatives.

// -----

// CHECK: creates an Arena with no backing and the empty-default capacity plan
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case0() {
    %a = "sec.arena.create_default"() <{state_after = 1 : i32, epoch_after = 3 : i32, backing_kind = "none", growth_kind = "fixed", capacity_plan = "borrowed-fixed-bytes", provider_plan = "absent", domain = 4 : i32, transition = "establish-domain", origin = "Arena", relation = "Owned", authority = "None", stability = "Stable", space = "ordinary"}> : () -> !sec.arena
    return
  }
}

// -----

// CHECK: an empty default Arena is Owned, retains no borrow and names its provider plan
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case1() {
    %a = "sec.arena.create_default"() <{state_after = 1 : i32, epoch_after = 3 : i32, backing_kind = "none", growth_kind = "fixed", capacity_plan = "empty-default", domain = 4 : i32, transition = "establish-domain", origin = "Arena", relation = "Owned", authority = "None", stability = "Stable", space = "ordinary"}> : () -> !sec.arena
    return
  }
}

// -----

// CHECK: an empty default Arena is Owned, retains no borrow and names its provider plan
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case2() {
    %a = "sec.arena.create_default"() <{state_after = 1 : i32, epoch_after = 3 : i32, backing_kind = "none", growth_kind = "fixed", capacity_plan = "empty-default", provider_plan = "absent", domain = 4 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "None", stability = "Stable", space = "ordinary"}> : () -> !sec.arena
    return
  }
}

// -----

// CHECK: creation establishes a fresh domain
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case3() {
    %a = "sec.arena.create_default"() <{state_after = 1 : i32, epoch_after = 3 : i32, backing_kind = "none", growth_kind = "fixed", capacity_plan = "empty-default", provider_plan = "absent", domain = 4 : i32, transition = "none", origin = "Arena", relation = "Owned", authority = "None", stability = "Stable", space = "ordinary"}> : () -> !sec.arena
    return
  }
}
