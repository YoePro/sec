// RUN: not sec-mlir-opt %s -split-input-file --sec-verify-arenas -o /dev/null 2>&1 | FileCheck %s

// SEC-MLIR-P19 § 158: Arena verifier negatives (shared/wrong-element backing,
// count/result types, epoch-changing allocation, Release without end-domain,
// reused domain, omitted parent dependency). Use after Release and double
// Release are in invalid-schema15-arena.mlir.

// CHECK: transfers one mutable byte-slice borrow into borrowed backing
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case0(%b: !sec.slice<ui8>) {
    %a = "sec.arena.create_borrowed"(%b) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice<ui8>) -> !sec.arena
    return
  }
}

// -----

// CHECK: transfers one mutable byte-slice borrow into borrowed backing
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case1(%b: !sec.slice_mut<!sec.int>) {
    %a = "sec.arena.create_borrowed"(%b) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice_mut<!sec.int>) -> !sec.arena
    return
  }
}

// -----

// CHECK: allocates uint count elements through a writable Arena Place into !sec.slice_mut<T>
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case2(%p: !sec.place<!sec.arena, "rw">, %n: !sec.int) {
    "sec.arena.alloc"(%p, %n) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 1 : i32, epoch_after = 1 : i32, backing_kind = "owned", growth_kind = "fixed", provider_plan = "provider", layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 9 : i32, transition = "none", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 9 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">, !sec.int) -> (!sec.slice_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    return
  }
}

// -----

// CHECK: allocates uint count elements through a writable Arena Place into !sec.slice_mut<T>
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case3(%p: !sec.place<!sec.arena, "rw">, %n: !sec.uint) {
    "sec.arena.alloc"(%p, %n) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 1 : i32, epoch_after = 1 : i32, backing_kind = "owned", growth_kind = "fixed", provider_plan = "provider", layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 9 : i32, transition = "none", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 9 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">, !sec.uint) -> (!sec.ref_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    return
  }
}

// -----

// CHECK: allocation advances the state version and preserves the validity epoch
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case4(%p: !sec.place<!sec.arena, "rw">, %n: !sec.uint) {
    "sec.arena.alloc"(%p, %n) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 1 : i32, epoch_after = 2 : i32, backing_kind = "owned", growth_kind = "fixed", provider_plan = "provider", layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 9 : i32, transition = "none", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 9 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">, !sec.uint) -> (!sec.slice_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    return
  }
}

// -----

// CHECK: Release ends the domain without publishing another epoch
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case5(%a: !sec.arena) {
    "sec.arena.release"(%a) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.arena) -> ()
    return
  }
}

// -----

// CHECK: must establish a fresh Arena domain
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case6(%b: !sec.slice_mut<ui8>, %c: !sec.slice_mut<ui8>) {
    %a = "sec.arena.create_borrowed"(%b) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice_mut<ui8>) -> !sec.arena
    %a2 = "sec.arena.create_borrowed"(%c) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice_mut<ui8>) -> !sec.arena
    return
  }
}

// -----

// CHECK: a nested child names parent storage, domain and epoch together
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @case7(%b: !sec.slice_mut<ui8>) {
    %a = "sec.arena.create_borrowed"(%b) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, parent_storage = 3 : i32, parent_domain = 4 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice_mut<ui8>) -> !sec.arena
    return
  }
}

// -----

// CHECK: Arena and allocation-context types require schema 15, module declares schema 14
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @regression(%c: !sec.alloc_context) {
    return
  }
}
