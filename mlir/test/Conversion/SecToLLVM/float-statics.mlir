// RUN: sec-mlir-opt %s --sec-lower-static-storage | FileCheck %s

// SEC-MLIR-P20 sections 74 and 76: a float static carries its canonical bit
// pattern and lowers to an LLVM global with exactly that payload.

// CHECK: llvm.mlir.global internal constant @half(5.000000e-01 : f64)
// CHECK: llvm.mlir.global internal constant @tiny(1.000000e-01 : f32)
// CHECK: llvm.mlir.global internal constant @negzero(-0.000000e+00 : f64)
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "bits:4602678819172646912", is_mutable = false, sym_name = "half", type = f64}> {sec.source_name = "Half", sec.static_id = 1 : i32} : () -> ()
  "sec.static.global"() <{initial = "bits:1036831949", is_mutable = false, sym_name = "tiny", type = f32}> {sec.source_name = "Tiny", sec.static_id = 2 : i32} : () -> ()
  "sec.static.global"() <{initial = "bits:9223372036854775808", is_mutable = false, sym_name = "negzero", type = f64}> {sec.source_name = "Negative", sec.static_id = 3 : i32} : () -> ()
}
