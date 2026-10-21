// RUN: sec-mlir-opt %s -verify-diagnostics -o /dev/null

// Shared and mutable slices cannot be interchanged by changing their types.
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @bad(%value: !sec.slice_mut<i32>) -> !sec.slice<i32> {
    // expected-error@+1 {{type of return operand 0 ('!sec.slice_mut<i32>') doesn't match function result type ('!sec.slice<i32>')}}
    return %value : !sec.slice_mut<i32>
  }
}
