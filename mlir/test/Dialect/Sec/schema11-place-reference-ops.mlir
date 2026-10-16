// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// Every schema-11 Place and direct-reference operation round-trips with its
// semantic metadata (sec_mlir_dialect_v11.md sections 8-26).
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema11-ops",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK-LABEL: func.func @places
  func.func @places(%value: !sec.int, %storage: !sec.storage<!sec.array<!sec.int, "3">>, %index: !sec.int) -> !sec.int {
    // CHECK: sec.place.value
    %root = "sec.place.value"(%value) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.int) -> !sec.place<!sec.int, "ro">
    // CHECK: sec.place.read
    %read = "sec.place.read"(%root) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
    // CHECK: sec.place.storage
    %array = "sec.place.storage"(%storage) <{place_root_id = 2 : i32, storage_identity = 2 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.array<!sec.int, "3">>) -> !sec.place<!sec.array<!sec.int, "3">, "rw">
    // CHECK: sec.place.array_index_in_bounds
    %in = "sec.place.array_index_in_bounds"(%array, %index) <{index_signed = true}> : (!sec.place<!sec.array<!sec.int, "3">, "rw">, !sec.int) -> i1
    // CHECK: sec.place.array_element
    %element = "sec.place.array_element"(%array, %index) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.int, "3">, "rw">, !sec.int) -> !sec.place<!sec.int, "rw">
    // CHECK: sec.place.write
    "sec.place.write"(%element, %read) : (!sec.place<!sec.int, "rw">, !sec.int) -> ()
    // CHECK: sec.ref.borrow_mut
    %mut = "sec.ref.borrow_mut"(%element) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
    // CHECK: sec.ref.move
    %moved = "sec.ref.move"(%mut) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<!sec.int>) -> !sec.ref_mut<!sec.int>
    // CHECK: sec.ref.reborrow_shared
    %shared = "sec.ref.reborrow_shared"(%moved) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref_mut<!sec.int>) -> !sec.ref<!sec.int>
    // CHECK: sec.ref.copy_shared
    %copy = "sec.ref.copy_shared"(%shared) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref<!sec.int>) -> !sec.ref<!sec.int>
    // CHECK: sec.ref.compare
    %same = "sec.ref.compare"(%shared, %copy) <{predicate = "eq"}> : (!sec.ref<!sec.int>, !sec.ref<!sec.int>) -> i1
    // CHECK: sec.place.deref
    %through = "sec.place.deref"(%copy) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
    // CHECK: sec.ref.reborrow_mut
    %again = "sec.ref.reborrow_mut"(%moved) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref_mut<!sec.int>) -> !sec.ref_mut<!sec.int>
    // CHECK: sec.ref.end_borrow
    "sec.ref.end_borrow"(%copy) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<!sec.int>) -> ()
    return %read : !sec.int
  }

  // CHECK-LABEL: func.func @dynamic
  func.func @dynamic(%reference: !sec.ref<!sec.int>) -> !sec.int {
    // CHECK: sec.ref.is_valid
    %valid = "sec.ref.is_valid"(%reference) : (!sec.ref<!sec.int>) -> i1
    cf.cond_br %valid, ^ok, ^stale
  ^ok:
    %place = "sec.place.deref"(%reference) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
    %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
    return %value : !sec.int
  ^stale:
    // CHECK: sec.fail.reference_generation
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
}
