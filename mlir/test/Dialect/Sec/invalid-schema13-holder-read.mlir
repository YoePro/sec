// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// P17: a holder read takes a reference holder out of its own Place with the
// facts of that read and only feeds reference operations; a mutable
// reference is never copied out of it as a value.

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "holder", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @no_facts(%storage: !sec.storage<!sec.ref<!sec.int>>) {
    %place = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.ref<!sec.int>>) -> !sec.place<!sec.ref<!sec.int>, "rw">
    // expected-error@+1 {{carries the sec.reference facts of its point}}
    %holder = "sec.place.read"(%place) <{action = "holder"}> : (!sec.place<!sec.ref<!sec.int>, "rw">) -> !sec.ref<!sec.int>
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "holder", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func private @take(!sec.ref_mut<!sec.int>) attributes {sec.function_id = "take", sec.source_name = "take", sec.extern = false, sec.unsafe = false}
  func.func @escape(%storage: !sec.storage<!sec.ref_mut<!sec.int>>) {
    %place = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.ref_mut<!sec.int>>) -> !sec.place<!sec.ref_mut<!sec.int>, "rw">
    // expected-error@+1 {{may only be validated, dereferenced, reborrowed, copied when shared or compared}}
    %holder = "sec.place.read"(%place) <{action = "holder"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.place<!sec.ref_mut<!sec.int>, "rw">) -> !sec.ref_mut<!sec.int>
    "sec.call.direct"(%holder) <{callee = @take}> {sec.argument_actions = ["mutable-reference"]} : (!sec.ref_mut<!sec.int>) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "holder", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @ref_of_ref(%r: !sec.ref<!sec.ref<!sec.int>>) {
    return
  }
}
// expected-error@-4 {{referent must be an ordinary value type}}
