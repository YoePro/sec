// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// v12 section 25A: Fill writes through a mutable slice with a value of its
// element type.

module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-fill", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @shared(%values: !sec.slice<si128>, %seed: si128) {
    // expected-error@+1 {{fill requires a mutable slice}}
    "sec.slice.fill"(%values, %seed) : (!sec.slice<si128>, si128) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-fill", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @mistyped(%values: !sec.slice_mut<si128>, %seed: si64) {
    // expected-error@+1 {{fill value must have the slice element type}}
    "sec.slice.fill"(%values, %seed) : (!sec.slice_mut<si128>, si64) -> ()
    return
  }
}
