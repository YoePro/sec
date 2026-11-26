// RUN: not sec-mlir-opt %s -split-input-file --sec-verify-arenas --sec-verify-allocation-contexts -o /dev/null 2>&1 | FileCheck %s

// SEC-MLIR-P19 sections 105-131: local contracts and the Arena and
// allocation-context verifiers.

// CHECK: Arena and allocation-context types require schema 15, module declares schema 14
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @old(%a: !sec.arena) {
    return
  }
}

// -----

// CHECK: Reset advances the state version and the validity epoch
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @flat_reset(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "checked", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
}

// -----

// CHECK: requires resolved dependency legality
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unproven_release(%a: !sec.arena) {
    "sec.arena.release"(%a) <{state_before = 3 : i32, state_after = 4 : i32, epoch_before = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "end-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.arena) -> ()
    return
  }
}

// -----

// CHECK: uses an Arena reference after Reset or Release invalidated its epoch
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @stale(%backing: !sec.slice_mut<ui8>, %p: !sec.place<!sec.arena, "rw">) {
    %a = "sec.arena.create_borrowed"(%backing) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice_mut<ui8>) -> !sec.arena
    %r, %f, %e = "sec.arena.new"(%p) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 5 : i32, transition = "none", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 5 : i32, epoch_dependency = 7 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">) -> (!sec.ref_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    "sec.arena.reset"(%p) <{epoch_advance = "checked", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 6 : i32, lifetime_id = 6 : i32}> : (!sec.ref_mut<!sec.int>) -> ()
    return
  }
}

// -----

// CHECK: releases an Arena owner that was already released
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @twice(%backing: !sec.slice_mut<ui8>) {
    %a = "sec.arena.create_borrowed"(%backing) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice_mut<ui8>) -> !sec.arena
    "sec.arena.release"(%a) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "end-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.arena) -> ()
    "sec.arena.release"(%a) <{state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "end-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.arena) -> ()
    return
  }
}

// -----

// CHECK: operand types must match callee inputs
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @needs(%ctx: !sec.alloc_context {sec.hidden_allocation_context = true}) attributes {sec.extern = false, sec.requires_allocation_context = true} {
    return
  }
  func.func @caller() {
    "sec.call.direct"() <{callee = @needs}> {sec.argument_actions = []} : () -> ()
    return
  }
}

// -----

// CHECK: is a foreign/extern ABI function and cannot gain a hidden allocation context
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @exported(%ctx: !sec.alloc_context {sec.hidden_allocation_context = true}) attributes {sec.extern = true, sec.requires_allocation_context = true} {
    return
  }
}

// -----

// CHECK: marks exactly its !sec.alloc_context argument as sec.hidden_allocation_context
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unmarked(%ctx: !sec.alloc_context) {
    return
  }
}
