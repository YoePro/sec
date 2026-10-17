// RUN: sec-mlir-opt %s > %t.first.mlir
// RUN: sec-mlir-opt %t.first.mlir > %t.second.mlir
// RUN: diff %t.first.mlir %t.second.mlir
// RUN: FileCheck %s --input-file=%t.first.mlir
// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -o /dev/null

// The schema-11 facts the emitter records outside operation properties
// round-trip and print deterministically: parameter and call-result
// sec.reference dictionaries, sec.return_origins, sec.address_taken and the
// discardable reference facts (implementation choice MD-084, P15-57/P15-58).
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema11-metadata",
  sec.source_files = ["metadata.sec"],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK-LABEL: func.func @sec_fn_0
  // CHECK-SAME: sec.reference = {address_stability = "Stable", borrow_id = 1 : i32, epoch_dependency = 2 : i32, invalidation_domain = 1 : i32, lifetime_id = 1 : i32, provenance = "arena", validity_policy = "dynamic-epoch"}
  // CHECK-SAME: sec.return_origins = ["param#0"]
  func.func @sec_fn_0(%v0: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "arena"}}) -> !sec.ref<!sec.int> attributes {sec.function_id = "main::Same(ref int)", sec.source_name = "Same", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["value"], sec.return_origins = ["param#0"]} {
    %v1 = "sec.ref.is_valid"(%v0) : (!sec.ref<!sec.int>) -> i1
    cf.cond_br %v1, ^bb1, ^bb2
  ^bb1:
    %v2 = "sec.place.deref"(%v0) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
    %v3 = "sec.ref.reborrow_shared"(%v2) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "arena", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
    return %v3 : !sec.ref<!sec.int>
  ^bb2:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }

  // CHECK-LABEL: func.func @sec_fn_1
  func.func @sec_fn_1() -> !sec.int attributes {sec.function_id = "main::Use()", sec.source_name = "Use", sec.extern = false, sec.unsafe = false} {
    %v0 = "sec.const.int"() <{value = "1"}> : () -> !sec.int
    // CHECK: sec.address_taken = true
    %s1 = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "slot", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.scalar_kind = "int", sec.address_taken = true} : () -> !sec.storage<!sec.int>
    "sec.storage.init"(%s1, %v0) : (!sec.storage<!sec.int>, !sec.int) -> ()
    %v1 = "sec.place.storage"(%s1) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
    // CHECK: sec.origin_storages = [1 : i32]
    // CHECK-SAME: sec.pinned = true
    // CHECK-SAME: sec.reference_address_space = "mmio"
    %v2 = "sec.ref.borrow_mut"(%v1) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> {sec.origin_storages = [1 : i32], sec.pinned = true, sec.reference_address_space = "mmio"} : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
    %v3 = "sec.ref.move"(%v2) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> {sec.origin_storages = [1 : i32]} : (!sec.ref_mut<!sec.int>) -> !sec.ref_mut<!sec.int>
    %v4 = "sec.ref.reborrow_shared"(%v3) <{borrow_id = 4 : i32, lifetime_id = 4 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 3 : i32}> : (!sec.ref_mut<!sec.int>) -> !sec.ref<!sec.int>
    // CHECK: sec.argument_actions = ["shared-reference"]
    // CHECK-SAME: sec.reference = {
    %v5 = "sec.call.direct"(%v4) <{callee = @sec_fn_2}> {sec.argument_actions = ["shared-reference"], sec.reference = {borrow_id = 5 : i32, lifetime_id = 5 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", origin_storages = [1 : i32]}} : (!sec.ref<!sec.int>) -> !sec.ref<!sec.int>
    %v6 = "sec.place.deref"(%v5) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
    %v7 = "sec.place.read"(%v6) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
    "sec.ref.end_borrow"(%v4) <{borrow_id = 4 : i32, lifetime_id = 4 : i32}> : (!sec.ref<!sec.int>) -> ()
    "sec.ref.end_borrow"(%v3) <{borrow_id = 3 : i32, lifetime_id = 3 : i32}> : (!sec.ref_mut<!sec.int>) -> ()
    return %v7 : !sec.int
  }

  // CHECK-LABEL: func.func @sec_fn_2
  func.func @sec_fn_2(%v0: !sec.ref<!sec.int> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.function_id = "main::Pass(ref int)", sec.source_name = "Pass", sec.extern = false, sec.unsafe = false, sec.return_origins = ["param#0"]} {
    return %v0 : !sec.ref<!sec.int>
  }
}
