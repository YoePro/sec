// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// collections.md section 7.9, v12 section 25A: Fill replaces every element of
// a mutable slice with one value and has no result.

// CHECK: "sec.slice.fill"(%{{.*}}, %{{.*}}) : (!sec.slice_mut<si128>, si128) -> ()
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-fill", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @fill(%values: !sec.slice_mut<si128>, %seed: si128) {
    "sec.slice.fill"(%values, %seed) : (!sec.slice_mut<si128>, si128) -> ()
    return
  }
}
