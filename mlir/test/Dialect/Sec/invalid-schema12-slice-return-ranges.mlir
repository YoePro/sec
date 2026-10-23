// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -split-input-file -verify-diagnostics -o /dev/null
// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -split-input-file -verify-diagnostics -o /dev/null
// P16 sections 66-67 and MD-084 section 4(6); no invented MD-115 spelling.
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
  func.func @shared_bounded(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %end = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %child : !sec.slice<si32>
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
  func.func @shared_wrapped(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %end = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    %wrapped = "sec.slice.copy_shared"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.slice<si32>) -> !sec.slice<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %wrapped : !sec.slice<si32>
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
  func.func private @pass(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.extern = false, sec.return_origins = ["param#0"]}
  func.func @shared_called(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %end = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    %called = "sec.call.direct"(%child) <{callee = @pass}> {sec.argument_actions = ["shared-reference"], sec.reference = {borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice<si32>) -> !sec.slice<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %called : !sec.slice<si32>
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
  func.func @shared_start_only(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start) <{start_present = true, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %child : !sec.slice<si32>
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
  func.func @shared_explicit_full(%source: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "0"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %total) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %child : !sec.slice<si32>
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
  func.func @mut_bounded(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice_mut<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice_mut<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %end = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_mut"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>, !sec.uint, !sec.uint) -> !sec.slice_mut<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %child : !sec.slice_mut<si32>
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
  func.func @mut_wrapped(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice_mut<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice_mut<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %end = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_mut"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>, !sec.uint, !sec.uint) -> !sec.slice_mut<si32>
    %wrapped = "sec.slice.move_mut"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %wrapped : !sec.slice_mut<si32>
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
  func.func private @pass(!sec.slice_mut<si32>) -> !sec.slice_mut<si32> attributes {sec.extern = false, sec.return_origins = ["param#0"]}
  func.func @mut_called(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice_mut<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice_mut<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %end = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_mut"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>, !sec.uint, !sec.uint) -> !sec.slice_mut<si32>
    %called = "sec.call.direct"(%child) <{callee = @pass}> {sec.argument_actions = ["mutable-reference"], sec.reference = {borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %called : !sec.slice_mut<si32>
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
  func.func @mut_start_only(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice_mut<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice_mut<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "1"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start) <{start_present = true, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_mut"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>, !sec.uint, !sec.uint) -> !sec.slice_mut<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %child : !sec.slice_mut<si32>
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
  func.func @mut_explicit_full(%source: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice_mut<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%source) : (!sec.slice_mut<si32>) -> !sec.uint
    %start = "sec.const.int"() <{value = "0"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total, %start, %total) <{start_present = true, end_present = true, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "contract"}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_mut"(%source, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>, !sec.uint, !sec.uint) -> !sec.slice_mut<si32>
    // expected-error@+1 {{returned sub-slice origin requires an explicit slice-range projection (MD-115)}}
    return %child : !sec.slice_mut<si32>
  }
}
