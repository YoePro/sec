// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// SEC-MLIR-P19 § 157: every schema-15 Arena and allocation-context
// operation and both types parse, print and verify, and round-trip.

// CHECK-LABEL: func.func @all
// CHECK: "sec.alloc_context.target"
// CHECK: "sec.alloc_context.compiler_local"
// CHECK: "sec.arena.create_owned_fixed"
// CHECK: "sec.arena.create_growable"
// CHECK: "sec.alloc_context.from_arena"
// CHECK: "sec.arena.alloc"
// CHECK: "sec.arena.destroy"
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @all(%cap: !sec.uint, %n: !sec.uint, %p: !sec.place<!sec.arena, "rw">, %owner: !sec.arena) attributes {sec.extern = false} {
    %t = "sec.alloc_context.target"() <{origin_plan = "target-entry"}> : () -> !sec.alloc_context
    %l = "sec.alloc_context.compiler_local"() <{origin_plan = "bounded-local"}> : () -> !sec.alloc_context
    %o, %of, %oe = "sec.arena.create_owned_fixed"(%cap, %t) <{state_after = 1 : i32, epoch_after = 1 : i32, backing_kind = "owned", growth_kind = "fixed", provider_plan = "provider", capacity_plan = "capacity", failure_atomic, domain = 9 : i32, transition = "establish-domain", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> : (!sec.uint, !sec.alloc_context) -> (!sec.arena, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    %g, %gf, %ge = "sec.arena.create_growable"(%cap, %l) <{state_after = 1 : i32, epoch_after = 1 : i32, backing_kind = "owned", growth_kind = "growable-nonrelocating", provider_plan = "provider", capacity_plan = "capacity", failure_atomic, domain = 10 : i32, transition = "establish-domain", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> : (!sec.uint, !sec.alloc_context) -> (!sec.arena, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    %x = "sec.alloc_context.from_arena"(%p) <{origin_plan = "explicit-arena", arena_domain = 9 : i32}> : (!sec.place<!sec.arena, "rw">) -> !sec.alloc_context
    %s, %sf, %se = "sec.arena.alloc"(%p, %n) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 1 : i32, epoch_after = 1 : i32, backing_kind = "owned", growth_kind = "fixed", provider_plan = "provider", layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 9 : i32, transition = "none", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 9 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">, !sec.uint) -> (!sec.slice_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    "sec.arena.destroy"(%owner) <{destruction_plan = "destroy-Arena", cause = "scope-exit", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 1 : i32, backing_kind = "owned", growth_kind = "fixed", provider_plan = "provider", dependency_proof = "no-live-dependency", domain = 11 : i32, transition = "end-domain", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> : (!sec.arena) -> ()
    return
  }
}
