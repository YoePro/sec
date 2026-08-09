// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module attributes {
  sec.dialect_version = 3 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "missing-layout",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {}

// CHECK: schema 3 requires explicit dlti.dl_spec
