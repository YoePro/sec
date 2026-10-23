// RUN: sec-mlir-opt %s --sec-verify-borrow-semantics -split-input-file -verify-diagnostics -o /dev/null
// Supplemental facts must never be hidden by a partial reference dictionary.
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @hidden_pinned(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact pinned}}
    %target = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.pinned = true, sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @hidden_origin_storages(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact origin_storages}}
    %target = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.origin_storages = [7 : i32], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @hidden_address_space(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact address_space}}
    %target = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.reference_address_space = "mmio", sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @wrong_source(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{holder transfer must preserve source_borrow}}
    %target = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 9 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @hidden_pinned(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact pinned}}
    %target = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.pinned = true, sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    return
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @hidden_origin_storages(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact origin_storages}}
    %target = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.origin_storages = [7 : i32], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    return
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @hidden_address_space(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{slice operation changes source reference fact address_space}}
    %target = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> {sec.reference_address_space = "mmio", sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    return
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-transfer-metadata",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @wrong_source(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    // expected-error@+1 {{holder transfer must preserve source_borrow}}
    %target = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 9 : i32}> : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    return
  }
}
