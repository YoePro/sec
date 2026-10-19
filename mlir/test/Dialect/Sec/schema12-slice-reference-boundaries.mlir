// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// P16-41 local operation contracts reuse P15's predicate and borrow-end
// marker. Full slice guard/borrow analysis is separate P16-42/P16-43 work.
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-reference-boundaries",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK-LABEL: func.func @shared(
  // CHECK: "sec.ref.is_valid"
  // CHECK: cf.cond_br
  // CHECK: "sec.ref.end_borrow"
  // CHECK-SAME: borrow_id = 1 : i32, lifetime_id = 2 : i32
  // CHECK: "sec.fail.reference_generation"
  // CHECK-SAME: panic.invalid-reference-generation
  func.func @shared(%view: !sec.slice<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 4 : i32, address_stability = "Stable", provenance = "arena"}}) {
    %valid = "sec.ref.is_valid"(%view) : (!sec.slice<!sec.int>) -> i1
    cf.cond_br %valid, ^live, ^stale
  ^live:
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.slice<!sec.int>) -> ()
    return
  ^stale:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
  // CHECK-LABEL: func.func @mutable(
  // CHECK: "sec.ref.is_valid"
  // CHECK: cf.cond_br
  // CHECK: "sec.ref.end_borrow"
  // CHECK-SAME: borrow_id = 1 : i32, lifetime_id = 2 : i32
  // CHECK: "sec.fail.reference_generation"
  // CHECK-SAME: panic.invalid-reference-generation
  func.func @mutable(%view: !sec.slice_mut<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 4 : i32, address_stability = "Stable", provenance = "arena"}}) {
    %valid = "sec.ref.is_valid"(%view) : (!sec.slice_mut<!sec.int>) -> i1
    cf.cond_br %valid, ^live, ^stale
  ^live:
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.slice_mut<!sec.int>) -> ()
    return
  ^stale:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
  // CHECK-LABEL: func.func @direct(
  // CHECK: "sec.ref.is_valid"
  // CHECK: cf.cond_br
  // CHECK: "sec.ref.end_borrow"
  // CHECK-SAME: borrow_id = 1 : i32, lifetime_id = 2 : i32
  // CHECK: "sec.fail.reference_generation"
  // CHECK-SAME: panic.invalid-reference-generation
  func.func @direct(%view: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 4 : i32, address_stability = "Stable", provenance = "arena"}}) {
    %valid = "sec.ref.is_valid"(%view) : (!sec.ref<!sec.int>) -> i1
    cf.cond_br %valid, ^live, ^stale
  ^live:
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^stale:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
  // CHECK-LABEL: func.func @fixed_array_index(
  // CHECK: "sec.fail.bounds"() {operation = "fixed-array-index"}
  func.func @fixed_array_index() {
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }
  // CHECK-LABEL: func.func @slice_index(
  // CHECK: "sec.fail.bounds"() {operation = "slice-index"}
  func.func @slice_index() {
    "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
  }
  // CHECK-LABEL: func.func @slice_range(
  // CHECK: "sec.fail.bounds"() {operation = "slice-range"}
  func.func @slice_range() {
    "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
  }
  func.func private @external_view() -> !sec.slice<i32>
  // CHECK-LABEL: func.func @call_view(
  // CHECK: call @external_view()
  // CHECK-SAME: sec.reference
  // CHECK: "sec.ref.end_borrow"
  func.func @call_view() {
    %view = func.call @external_view() {sec.reference = {borrow_id = 1 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "external-contract", address_stability = "Stable", provenance = "parameter"}} : () -> !sec.slice<i32>
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.slice<i32>) -> ()
    return
  }
}
