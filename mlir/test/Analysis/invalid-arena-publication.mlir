// RUN: not sec-mlir-opt --split-input-file --sec-verify-arenas %s 2>&1 | FileCheck %s

// P19 §§ 35, 151 and v15 § 14: a checked New/Alloc (and owned creation)
// branches on its own failed flag and publishes its candidate only on the
// success edge; a failed allocation never exposes a partial reference.

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @no_branch(%p: !sec.place<!sec.arena, "rw">) {
    %r, %f, %e = "sec.arena.new"(%p) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 5 : i32, transition = "none", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 5 : i32, epoch_dependency = 7 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">) -> (!sec.ref_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    return
  }
}
// CHECK: must end its block with the branch on its own failed flag before publishing its candidate

// -----

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @failure_edge(%p: !sec.place<!sec.arena, "rw">) {
    %r, %f, %e = "sec.arena.new"(%p) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 5 : i32, transition = "none", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 5 : i32, epoch_dependency = 7 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">) -> (!sec.ref_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %f, ^failed(%r : !sec.ref_mut<!sec.int>), ^allocated
  ^failed(%x: !sec.ref_mut<!sec.int>):
    return
  ^allocated:
    return
  }
}
// CHECK: passes its candidate across the allocation failure edge

// -----

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @join_use(%p: !sec.place<!sec.arena, "rw">) {
    %r, %f, %e = "sec.arena.new"(%p) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 5 : i32, transition = "none", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 5 : i32, epoch_dependency = 7 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">) -> (!sec.ref_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %f, ^failed, ^allocated
  ^failed:
    cf.br ^join
  ^allocated:
    cf.br ^join
  ^join:
    cf.br ^done(%r : !sec.ref_mut<!sec.int>)
  ^done(%y: !sec.ref_mut<!sec.int>):
    return
  }
}
// CHECK: uses an Arena allocation candidate where the allocation may have failed

// -----

module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @shared_success(%p: !sec.place<!sec.arena, "rw">) {
    %r, %f, %e = "sec.arena.new"(%p) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 5 : i32, transition = "none", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 5 : i32, epoch_dependency = 7 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">) -> (!sec.ref_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %f, ^allocated, ^allocated
  ^allocated:
    return
  }
}
// CHECK: success edge must be the only way into its publication block
