// RUN: not sec-mlir-opt %s -split-input-file -o /dev/null 2>&1 | FileCheck %s

// v14 section 13B: Clear changes the owner and needs a writable owner Place.

// CHECK: 'sec.dynamic_array.clear' op changes the owner and requires a writable Place
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @readonly() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "ro">
    "sec.dynamic_array.clear"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "ro">) -> ()
    return
  }
}

// -----

// v14 section 13B: an element that needs destruction requires its plan.
// RUN: not sec-mlir-opt %s -split-input-file --sec-verify-destruction-plans -o /dev/null 2>&1 | FileCheck %s --check-prefix=PLAN
// PLAN: 'sec.dynamic_array.clear' op clears elements that need destruction without their plan
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @nested() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.dynamic_array<!sec.int>>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.dynamic_array<!sec.int>>>) -> !sec.place<!sec.dynamic_array<!sec.dynamic_array<!sec.int>>, "rw">
    "sec.dynamic_array.clear"(%p) : (!sec.place<!sec.dynamic_array<!sec.dynamic_array<!sec.int>>, "rw">) -> ()
    return
  }
}
