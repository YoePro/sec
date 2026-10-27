// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// MD-115 §§2,6,10: exact decimal endpoints round-trip beyond host widths.
// CHECK: param#0[1..<3]
// CHECK: param#0[0..<0]
// CHECK: param#0[?]
// CHECK: param#0[184467440737095516160..<184467440737095516170]
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func private @exact(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[1..<3]"]}
  func.func private @empty(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[0..<0]"]}
  func.func private @unknown(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[?]"]}
  func.func private @huge(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[184467440737095516160..<184467440737095516170]"]}
}
