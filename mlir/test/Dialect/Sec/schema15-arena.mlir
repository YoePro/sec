// RUN: sec-mlir-opt %s --sec-verify-arenas --sec-verify-allocation-contexts | FileCheck %s

// SEC-MLIR-P19 sections 105-123: a borrowed Arena is created, allocates one
// T with an epoch-preserving state step, resets (advancing its epoch) and is
// released; an owned Arena is created through the hidden allocation context,
// which a direct call forwards. Each checked op branches on its own failed
// flag and publishes its candidate only on the success edge (sections 35, 151).

// CHECK-LABEL: func.func @borrowed
// CHECK-SAME: kind = "unbounded"
// CHECK: "sec.arena.create_borrowed"
// CHECK: "sec.arena.new"
// CHECK: cf.cond_br
// CHECK: "sec.arena.reset"
// CHECK: "sec.arena.release"
// CHECK-LABEL: func.func @owned
// CHECK: "sec.arena.create_owned_fixed"
// CHECK: "sec.call.direct"
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @borrowed(%backing: !sec.slice_mut<ui8>, %p: !sec.place<!sec.arena, "rw">) attributes {sec.arena_demand_summary = [{domain = 5 : i32, expression = "unbounded(recursion through Deep)", kind = "unbounded"}]} {
    %a = "sec.arena.create_borrowed"(%backing) <{state_after = 1 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, domain = 5 : i32, transition = "establish-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.slice_mut<ui8>) -> !sec.arena
    %r, %f, %e = "sec.arena.new"(%p) <{state_before = 1 : i32, state_after = 2 : i32, epoch_before = 7 : i32, epoch_after = 7 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, layout_plan = "layout", default_plan = "default", capacity_plan = "capacity", failure_atomic, domain = 5 : i32, transition = "none", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 5 : i32, epoch_dependency = 7 : i32, address_stability = "Stable", provenance = "arena"}} : (!sec.place<!sec.arena, "rw">) -> (!sec.ref_mut<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %f, ^failed, ^allocated
  ^failed:
    return
  ^allocated:
    "sec.arena.reset"(%p) <{epoch_advance = "checked", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    "sec.arena.release"(%a) <{state_before = 3 : i32, state_after = 4 : i32, epoch_before = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "end-domain", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.arena) -> ()
    return
  }
  func.func @owned(%ctx: !sec.alloc_context {sec.hidden_allocation_context = true}, %cap: !sec.uint) attributes {sec.extern = false, sec.requires_allocation_context = true} {
    %o, %of, %oe = "sec.arena.create_owned_fixed"(%cap, %ctx) <{state_after = 1 : i32, epoch_after = 1 : i32, backing_kind = "owned", growth_kind = "fixed", provider_plan = "provider", capacity_plan = "capacity", failure_atomic, domain = 9 : i32, transition = "establish-domain", origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Stable", space = "ordinary"}> : (!sec.uint, !sec.alloc_context) -> (!sec.arena, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %of, ^failed, ^created
  ^failed:
    return
  ^created:
    "sec.call.direct"(%ctx, %cap) <{callee = @owned}> {sec.argument_actions = ["allocation-context", "copy-trivial"]} : (!sec.alloc_context, !sec.uint) -> ()
    return
  }
}
