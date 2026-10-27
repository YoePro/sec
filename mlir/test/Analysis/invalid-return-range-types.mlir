// RUN: sec-mlir-opt %s -split-input-file --sec-verify-places --sec-verify-borrow-semantics -verify-diagnostics -o /dev/null
// MD-115 §3: grammar alone cannot grant sequence type or mutable authority.
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
  // expected-error@+1 {{returned mutable slice requires mutable origin authority}}
  func.func @authority(%shared: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %actual: !sec.slice_mut<si32> {sec.reference = {borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice_mut<si32> attributes {sec.return_origins = ["param#0[?]"]} {
    return %actual : !sec.slice_mut<si32>
  }
}

// -----

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
  // expected-error@+1 {{returned range has an inadmissible typed projection}}
  func.func @scalar(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<si32> attributes {sec.return_origins = ["param#0[?][*]"]} {
    return %source : !sec.ref<si32>
  }
}
