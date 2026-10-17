// RUN: not sec-mlir-opt %s --sec-lower-trivial-core 2>&1 | FileCheck %s

// P15-59: the address-taken marker exists only on high-level semantic
// storage, so reference-dependent storage cannot appear as a MemRef.
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "lowered", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @lowered() attributes {sec.extern = false} {
    // CHECK: sec.address_taken annotates sec.storage.declare
    %storage = memref.alloca() {sec.storage_id = 1 : i32, sec.address_taken = true} : memref<i32>
    return
  }
}
