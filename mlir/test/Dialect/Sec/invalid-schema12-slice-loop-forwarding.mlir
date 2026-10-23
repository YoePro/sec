// RUN: sec-mlir-opt %s --sec-verify-reference-guards -split-input-file -verify-diagnostics -o /dev/null
// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-verify-reference-guards -split-input-file -verify-diagnostics -o /dev/null
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
  func.func @changed_source(%flag: i1, %first: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}, %second: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}) {
    cf.br ^loop(%first : !sec.slice<si32>)
  ^loop(%carried: !sec.slice<si32>):
    // expected-error@+1 {{slice observation has no unambiguous reference facts}}
    %len = "sec.slice.len"(%carried) : (!sec.slice<si32>) -> !sec.uint
    cf.cond_br %flag, ^loop(%second : !sec.slice<si32>), ^exit
  ^exit:
    return
  }
}

// -----

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
  func.func @rootless() {
    return
  ^loop(%carried: !sec.slice<si32>):
    // expected-error@+1 {{slice observation has no unambiguous reference facts}}
    %len = "sec.slice.len"(%carried) : (!sec.slice<si32>) -> !sec.uint
    cf.br ^loop(%carried : !sec.slice<si32>)
  }
}

// -----

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
  func.func @missing_guard(%flag: i1, %source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %other: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^loop(%source : !sec.slice<si32>)
  ^loop(%carried: !sec.slice<si32>):
    // expected-error@+1 {{dynamic slice observation requires a dominating same-source validity guard}}
    %len = "sec.slice.len"(%carried) : (!sec.slice<si32>) -> !sec.uint
    cf.cond_br %flag, ^loop(%carried : !sec.slice<si32>), ^exit
  ^exit:
    return
  }
}

// -----

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
  func.func @missing_guard(%flag: i1, %source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %other: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "parameter", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^loop(%source : !sec.slice<si32>)
  ^loop(%carried: !sec.slice<si32>):
    %valid = "sec.ref.is_valid"(%other) : (!sec.slice<si32>) -> i1
    cf.cond_br %valid, ^live, ^stale
  ^live:
    // expected-error@+1 {{dynamic slice observation requires a dominating same-source validity guard}}
    %len = "sec.slice.len"(%carried) : (!sec.slice<si32>) -> !sec.uint
    cf.cond_br %flag, ^loop(%carried : !sec.slice<si32>), ^exit
  ^exit:
    return
  ^stale:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
}
