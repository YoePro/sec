// RUN: sec-mlir-opt %s -split-input-file --sec-verify-borrow-semantics -verify-diagnostics -o /dev/null
// RUN: sec-mlir-opt %s -split-input-file --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -verify-diagnostics -o /dev/null
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md §§39–46,60–63,83–86,116.
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_borrow_id(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice transfer/element Place changes source borrow_id}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 4 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 4 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_lifetime_id(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice transfer/element Place changes source lifetime_id}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 4 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_epoch_dependency(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact epoch_dependency}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 8 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_invalidation_domain(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact invalidation_domain}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 8 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_address_stability(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact address_stability}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Unknown", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_provenance(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact provenance}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wrong_parent(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %total = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    // expected-error@+1 {{source_borrow must name}}
    %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 9 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wrong_end(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice end_borrow must match}}
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si32>) -> ()
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @after_end(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %length = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @after_move(%view: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %copy = "sec.slice.move_mut"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    // expected-error@+1 {{slice used after mutable move}}
    %length = "sec.slice.len"(%view) : (!sec.slice_mut<si32>) -> !sec.uint
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @forwarded_after_end(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    cf.br ^alias(%view : !sec.slice<si32>)
    ^alias(%alias: !sec.slice<si32>):
    "sec.ref.end_borrow"(%alias) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %length = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @parent_ended_before_child(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %total = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %length = "sec.slice.len"(%child) : (!sec.slice<si32>) -> !sec.uint
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @parent_ended_before_copied_child(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %total = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    %other = "sec.slice.copy_shared"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 2 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %length = "sec.slice.len"(%other) : (!sec.slice<si32>) -> !sec.uint
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @loop_cannot_revive_parameter(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %again: i1) {
    cf.br ^loop
    ^loop:
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %length = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    cf.cond_br %again, ^loop, ^done
    ^done:
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @element_after_end(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %i = "sec.const.int"() <{value = "0"}> : () -> !sec.uint
    %p = "sec.slice.element_place"(%view, %i) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = false, bounds_kind = "proven-safe", bounds_proof = "contract"}> : (!sec.slice<si32>, !sec.uint) -> !sec.place<si32, "ro">
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %v = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<si32, "ro">) -> si32
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @element_changes_epoch(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %i = "sec.const.int"() <{value = "0"}> : () -> !sec.uint
    // expected-error@+1 {{slice operation changes source reference fact validity_policy}}
    %p = "sec.slice.element_place"(%view, %i) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", index_signed = false, bounds_kind = "proven-safe", bounds_proof = "contract"}> : (!sec.slice<si32>, !sec.uint) -> !sec.place<si32, "ro">
    return
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{a function returning a direct reference requires sec.return_origins}}
  func.func @missing_return_summary(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> {
    return %view : !sec.slice<si32>
  }
}
// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wrong_return_root(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    // expected-error@+1 {{which sec.return_origins does not list}}
    return %other : !sec.slice<si32>
  }
}

// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_pin(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", pinned = true}}) {
    // expected-error@+1 {{changes source reference fact pinned}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}>  : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_storage(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", origin_storages = [7 : i32]}}) {
    // expected-error@+1 {{changes source reference fact origin_storages}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.origin_storages = [8 : i32]} : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @changed_space(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", address_space = "mmio"}}) {
    // expected-error@+1 {{changes source reference fact address_space}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}>  : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @local_return(%array: !sec.array<si32, "4">, %external: !sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#1"]} {
    %place = "sec.place.value"(%array) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.array<si32, "4">) -> !sec.place<!sec.array<si32, "4">, "ro">
    %four = "sec.const.int"() <{value = "4"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%four) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %view = "sec.slice.borrow_shared"(%place, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.array<si32, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si32>
    // expected-error@+1 {{returns a reference to local storage or a local value}}
    return %view : !sec.slice<si32>
  }
}

// -----
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wrong_owned_storage(%array: !sec.array<si32, "4">) {
    %place = "sec.place.value"(%array) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.array<si32, "4">) -> !sec.place<!sec.array<si32, "4">, "ro">
    %four = "sec.const.int"() <{value = "4"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%four) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    // expected-error@+1 {{fixed-array slice borrow changes its Place storage identity}}
    %view = "sec.slice.borrow_shared"(%place, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> {sec.origin_storages = [9 : i32]} : (!sec.place<!sec.array<si32, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si32>
    return
  }
}

// -----

module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @invented_range(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice transfer changes represented range facts}}
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.slice_range_id = 99 : i32} : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

