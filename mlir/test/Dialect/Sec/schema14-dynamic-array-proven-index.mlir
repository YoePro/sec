// RUN: sec-mlir-opt %s --sec-verify-dynamic-arrays --sec-verify-storage-transitions --sec-verify-places | FileCheck %s

// SEC-MLIR-P18 section 41: an element Place whose index the frontend proved
// below the runtime length (a dominating index < values.Len branch or
// assertion) needs no guard.

// CHECK: bounds_kind = "proven-safe", bounds_proof = "branch"
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @proven(%i: !sec.uint) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    %e = "sec.dynamic_array.element_place"(%p, %i) <{index_signed = false, bounds_kind = "proven-safe", bounds_proof = "branch", backing_domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.uint) -> !sec.place<!sec.int, "rw">
    return
  }
}
