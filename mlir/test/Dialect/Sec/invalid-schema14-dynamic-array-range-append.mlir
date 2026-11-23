// RUN: not sec-mlir-opt %s -split-input-file --sec-verify-dynamic-arrays -o /dev/null 2>&1 | FileCheck %s

// SEC-MLIR-P18 P18-17: the ensured capacity must cover the appended count,
// and the segment needs a positive count and an element-typed lower bound.

// CHECK: appends 9 values but the ensured capacity covers 1
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @short(%lower: !sec.int) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    %spare = "sec.dynamic_array.has_capacity"(%p) <{count = "1"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    cf.cond_br %spare, ^append, ^done
  ^append:
    "sec.dynamic_array.append_range_prepared"(%p, %lower) <{count = "9"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> ()
    return
  ^done:
    return
  }
}

// -----

// CHECK: requires a positive canonical value count
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @zero(%lower: !sec.int) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    "sec.dynamic_array.append_range_prepared"(%p, %lower) <{count = "0"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> ()
    return
  }
}

// -----

// CHECK: takes a lower bound of the array's element type
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @mistyped(%lower: !sec.uint) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    "sec.dynamic_array.append_range_prepared"(%p, %lower) <{count = "2"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.uint) -> ()
    return
  }
}
