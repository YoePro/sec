// RUN: sec-mlir-opt %s --sec-verify-arenas | FileCheck %s

// MD-136 (P19 § 166): the empty default Arena, an owned domain with no
// backing and the empty-default capacity plan.

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @fresh() {
    %a = "sec.arena.create_default"() <{state_after = 1 : i32, epoch_after = 3 : i32, backing_kind = "none", growth_kind = "fixed", capacity_plan = "empty-default", provider_plan = "absent", domain = 4 : i32, transition = "establish-domain", origin = "Arena", relation = "Owned", authority = "None", stability = "Stable", space = "ordinary"}> : () -> !sec.arena
    return
  }
}
// CHECK: "sec.arena.create_default"
// CHECK-SAME: backing_kind = "none"
