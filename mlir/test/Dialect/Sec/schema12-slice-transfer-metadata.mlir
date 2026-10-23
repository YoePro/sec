// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | sec-mlir-opt --sec-verify-borrow-semantics -o /dev/null
// MD-084 sections 2(5),3(3)-(4), inherited by schema 12.
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
  func.func @copy_shared_omitted(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %target = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
  func.func @copy_shared_explicit(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %target = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
  func.func @copy_shared_supplemental(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", pinned = true, origin_storages = [7 : i32], address_space = "mmio"}}) {
    %target = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.pinned = true, sec.origin_storages = [7 : i32], sec.reference_address_space = "mmio", sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice<si32>) -> !sec.slice<si32>
    return
  }
  func.func @move_mut_omitted(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %target = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    return
  }
  func.func @move_mut_explicit(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %target = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    return
  }
  func.func @move_mut_supplemental(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", pinned = true, origin_storages = [7 : i32], address_space = "mmio"}}) {
    %target = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.pinned = true, sec.origin_storages = [7 : i32], sec.reference_address_space = "mmio", sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    return
  }
}
