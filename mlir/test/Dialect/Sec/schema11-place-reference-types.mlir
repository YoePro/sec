// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// Schema 11 (SEC-MLIR-P15) Place and direct reference types round-trip and
// are accepted in function signatures; schemas 9 and 10 stay accepted
// (schema9-structs.mlir, schema10-module-metadata.mlir). Schema 12 is now
// accepted by P16-38; unknown schema 13 is rejected.
// CHECK: sec.dialect_version = 11 : i32
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema11",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK-LABEL: func.func private @shared(!sec.ref<!sec.int>) -> !sec.ref<!sec.int>
  func.func private @shared(%value: !sec.ref<!sec.int>) -> !sec.ref<!sec.int>
  // CHECK-LABEL: func.func private @mutable(!sec.ref_mut<!sec.array<i32, "3">>)
  func.func private @mutable(%value: !sec.ref_mut<!sec.array<i32, "3">>)
  // CHECK-LABEL: func.func private @places(!sec.place<!sec.int, "ro">, !sec.place<!sec.int, "rw">)
  func.func private @places(%read: !sec.place<!sec.int, "ro">, %write: !sec.place<!sec.int, "rw">)
}

