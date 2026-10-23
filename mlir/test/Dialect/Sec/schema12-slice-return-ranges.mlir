// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | sec-mlir-opt --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -o /dev/null
// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -o /dev/null
// Whole-slice returns and element [*] projections retain their existing grammar.
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @shared_whole(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice<si32>) -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    cf.br ^exit(%child : !sec.slice<si32>)
  ^exit(%forwarded: !sec.slice<si32>):
    return %forwarded : !sec.slice<si32>
  }
  func.func @element_origin(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<si32> attributes {sec.return_origins = ["param#0[*]"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %end = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.uint
    %place = "sec.slice.element_place"(%child, %zero) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32, index_signed = false, bounds_kind = "proven-safe", bounds_proof = "contract"}> : (!sec.slice<si32>, !sec.uint) -> !sec.place<si32, "ro">
    %ref = "sec.ref.borrow_shared"(%place) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 2 : i32}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return %ref : !sec.ref<si32>
  }
  func.func @mut_whole(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice_mut<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice_mut<si32>) -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_mut"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>, !sec.uint, !sec.uint) -> !sec.slice_mut<si32>
    cf.br ^exit(%child : !sec.slice_mut<si32>)
  ^exit(%forwarded: !sec.slice_mut<si32>):
    return %forwarded : !sec.slice_mut<si32>
  }
}
