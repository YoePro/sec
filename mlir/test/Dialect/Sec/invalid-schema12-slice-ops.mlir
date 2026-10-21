// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// P16-40: local contracts reject malformed schemas, carriers, authority,
// metadata and error classes without requiring future CFG analysis.

// old scalar-only range schema
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{slice operations require schema 12}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    return
  }
}

// -----

// missing schema
module {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{slice operations require schema 12}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    return
  }
}

// -----

// missing endpoint
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{bound presence must match}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = true, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "constant"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    return
  }
}

// -----

// omitted signed endpoint
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{an omitted endpoint has no signedness}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = true, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    return
  }
}

// -----

// wrong endpoint signedness
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint, %i: si128) {
    // expected-error@+1 {{endpoint signedness must match}}
    %s, %l = "sec.slice.range_normalize"(%n, %i) <{start_present = true, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "constant"}> : (!sec.uint, si128) -> (!sec.uint, !sec.uint)
    return
  }
}

// -----

// bad proof
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{range normalization requires a resolved proof_kind}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "guess"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    return
  }
}

// -----

// open proof with endpoint
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint, %i: si128) {
    // expected-error@+1 {{source-open-bound proof requires both endpoints omitted}}
    %s, %l = "sec.slice.range_normalize"(%n, %i) <{start_present = true, end_present = false, end_exclusive = true, start_signed = true, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint, si128) -> (!sec.uint, !sec.uint)
    return
  }
}

// -----

// signed count
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.int) {
    // expected-error@+1 {{same uint count type}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.int) -> (!sec.int, !sec.int)
    return
  }
}

// -----

// wrong resolved count width
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: ui32) {
    // expected-error@+1 {{same uint count type}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (ui32) -> (ui32, ui32)
    return
  }
}

// -----

// mixed normalized output types
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{same uint count type}}
    %s, %l = "sec.slice.range_normalize"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, ui64)
    return
  }
}

// -----

// array value is not a Place
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.array<si128, "4">, %n: !sec.uint) {
    // expected-error@+1 {{fixed-array Place}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}>  : (!sec.array<si128, "4">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// mutable borrow from readonly
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{cannot increase authority}}
    %s = "sec.slice.borrow_mut"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}>  : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
    return
  }
}

// -----

// changed element
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{preserve element type}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}>  : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si256>
    return
  }
}

// -----

// zero borrow identity
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{borrow_id and lifetime_id must be non-zero}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 0 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}>  : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// fresh borrow with source identity
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{fresh slice borrow has no source_borrow}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 2 : i32}>  : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// reborrow from direct ref
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.ref<si128>, %n: !sec.uint) {
    // expected-error@+1 {{reborrow source must be a bounded slice}}
    %s = "sec.slice.reborrow_shared"(%source, %n, %n) <{borrow_id = 2 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}>  : (!sec.ref<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// mutable reborrow from shared
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>, %n: !sec.uint) {
    // expected-error@+1 {{cannot increase authority}}
    %s = "sec.slice.reborrow_mut"(%source, %n, %n) <{borrow_id = 2 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}>  : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
    return
  }
}

// -----

// same reborrow identity
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>, %n: !sec.uint) {
    // expected-error@+1 {{distinct non-zero source_borrow}}
    %s = "sec.slice.reborrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}>  : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// copy_shared invalid holder
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice_mut<si128>) {
    // expected-error@+1 {{matching slice type and authority}}
    %s = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si128>) -> !sec.slice_mut<si128>
    return
  }
}

// -----

// move_mut invalid holder
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>) {
    // expected-error@+1 {{matching slice type and authority}}
    %s = "sec.slice.move_mut"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>) -> !sec.slice<si128>
    return
  }
}

// -----

// copy_shared invalid holder
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>) {
    // expected-error@+1 {{preserve source_borrow}}
    %s = "sec.slice.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.slice<si128>) -> !sec.slice<si128>
    return
  }
}

// -----

// signed length
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>) {
    // expected-error@+1 {{uint count result}}
    %n = "sec.slice.len"(%source) : (!sec.slice<si128>) -> !sec.int
    return
  }
}

// -----

// wrong resolved length
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>) {
    // expected-error@+1 {{uint count result}}
    %n = "sec.slice.len"(%source) : (!sec.slice<si128>) -> ui32
    return
  }
}

// -----

// direct ref length
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.ref<si128>) {
    // expected-error@+1 {{bounded slice}}
    %n = "sec.slice.len"(%source) : (!sec.ref<si128>) -> !sec.uint
    return
  }
}

// -----

// index signedness
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>, %i: si256) {
    // expected-error@+1 {{index_signed must match}}
    %ok = "sec.slice.index_in_bounds"(%source, %i) <{index_signed = false}> : (!sec.slice<si128>, si256) -> i1
    return
  }
}

// -----

// signless index
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>, %i: i256) {
    // expected-error@+1 {{index_signed must match}}
    %ok = "sec.slice.index_in_bounds"(%source, %i) <{index_signed = false}> : (!sec.slice<si128>, i256) -> i1
    return
  }
}

// -----

// element Place rwguarded
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>, %i: si128) {
    // expected-error@+1 {{preserve slice element type and authority}}
    %p = "sec.slice.element_place"(%source, %i) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si128) -> !sec.place<si128, "rw">
    return
  }
}

// -----

// element Place roconstant
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.slice<si128>, %i: si128) {
    // expected-error@+1 {{runtime-check bounds proof must be guarded}}
    %p = "sec.slice.element_place"(%source, %i) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "constant"}> : (!sec.slice<si128>, si128) -> !sec.place<si128, "ro">
    return
  }
}

// -----

// range provenance
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{canonical non-negative decimal strings}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.slice_static_start = "01"} : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// range provenance
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{canonical non-negative decimal strings}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.slice_static_start = "-1"} : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// range provenance
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{static slice start exceeds its exclusive end}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.slice_static_start = "184467440737095516170", sec.slice_static_end_exclusive = "184467440737095516169"} : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// range provenance
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%source: !sec.place<!sec.array<si128, "4">, "ro">, %n: !sec.uint) {
    // expected-error@+1 {{slice_range_id must be a non-zero i32}}
    %s = "sec.slice.borrow_shared"(%source, %n, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.slice_range_id = 0 : i32} : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    return
  }
}

// -----

// canonical range error
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{canonical core::RangeError}}
    %s, %l, %f, %e = "sec.slice.range_check"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false}> : (!sec.uint) -> (!sec.uint, !sec.uint, i1, !sec.enum<identity = "user::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>)
    return
  }
}

// -----

// canonical range error
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-slice-op", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%n: !sec.uint) {
    // expected-error@+1 {{canonical core::RangeError}}
    %s, %l, %f, %e = "sec.slice.range_check"(%n) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false}> : (!sec.uint) -> (!sec.uint, !sec.uint, i1, !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "3">]>)
    return
  }
}
