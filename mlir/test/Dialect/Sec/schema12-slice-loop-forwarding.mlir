// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | sec-mlir-opt --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -o /dev/null
// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -o /dev/null
// P16 sections 83-86: unchanged loop-carried slice/normalization identity.
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-loop-forwarding",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @shared_proven(%flag: i1, %source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}) {
    cf.br ^loop(%source : !sec.slice<si32>)
  ^loop(%carried: !sec.slice<si32>):
    %len = "sec.slice.len"(%carried) : (!sec.slice<si32>) -> !sec.uint
    cf.cond_br %flag, ^loop(%carried : !sec.slice<si32>), ^exit
  ^exit:
    return
  }
  func.func @shared_dynamic(%flag: i1, %source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^loop(%source : !sec.slice<si32>)
  ^loop(%carried: !sec.slice<si32>):
    %valid = "sec.ref.is_valid"(%carried) : (!sec.slice<si32>) -> i1
    cf.cond_br %valid, ^live, ^stale
  ^live:
    %len = "sec.slice.len"(%carried) : (!sec.slice<si32>) -> !sec.uint
    cf.cond_br %flag, ^loop(%carried : !sec.slice<si32>), ^exit
  ^exit:
    return
  ^stale:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
  func.func @mutable_proven(%flag: i1, %source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}) {
    cf.br ^loop(%source : !sec.slice_mut<si32>)
  ^loop(%carried: !sec.slice_mut<si32>):
    %len = "sec.slice.len"(%carried) : (!sec.slice_mut<si32>) -> !sec.uint
    cf.cond_br %flag, ^loop(%carried : !sec.slice_mut<si32>), ^exit
  ^exit:
    return
  }
  func.func @mutable_dynamic(%flag: i1, %source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^loop(%source : !sec.slice_mut<si32>)
  ^loop(%carried: !sec.slice_mut<si32>):
    %valid = "sec.ref.is_valid"(%carried) : (!sec.slice_mut<si32>) -> i1
    cf.cond_br %valid, ^live, ^stale
  ^live:
    %len = "sec.slice.len"(%carried) : (!sec.slice_mut<si32>) -> !sec.uint
    cf.cond_br %flag, ^loop(%carried : !sec.slice_mut<si32>), ^exit
  ^exit:
    return
  ^stale:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
  func.func @range_tuple(%flag: i1, %array: !sec.array<si32, "4">) {
    %place = "sec.place.value"(%array) <{place_root_id = 1 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : (!sec.array<si32, "4">) -> !sec.place<!sec.array<si32, "4">, "ro">
    %total = "sec.const.int"() <{value = "4"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    cf.br ^loop(%s, %n : !sec.uint, !sec.uint)
  ^loop(%start: !sec.uint, %length: !sec.uint):
    %view = "sec.slice.borrow_shared"(%place, %start, %length) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}> : (!sec.place<!sec.array<si32, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si32>
    %len = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    cf.cond_br %flag, ^loop(%start, %length : !sec.uint, !sec.uint), ^exit
  ^exit:
    return
  }
}
