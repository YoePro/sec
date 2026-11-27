// RUN: sec-mlir-opt %s --sec-verify-dynamic-arrays --sec-verify-places | FileCheck %s

// collections.md section 3.4, v14 section 13D: instance SizeOf is Len times
// the element stride, observed through a read-only owner Place.

// CHECK: "sec.dynamic_array.payload_size"(%{{.*}}) <{stride = "8"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "ro">) -> !sec.uint
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @size() -> !sec.uint {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "ro">
    %n = "sec.dynamic_array.payload_size"(%p) <{stride = "8"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "ro">) -> !sec.uint
    return %n : !sec.uint
  }
}
