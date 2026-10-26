// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-lower-floats | FileCheck %s
// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s --check-prefix=LLVM
// P20 §23: exact truncation remainder, including subnormal results, uses
// bounded integer reduction rather than an undeclared math-runtime helper.
// CHECK-LABEL: func.func @remainder32
// CHECK: arith.cmpi uge
// CHECK: cf.cond_br
// CHECK: arith.shli
// CHECK: arith.shrui
// CHECK-LABEL: func.func @remainder64
// CHECK: arith.cmpi uge
// CHECK: arith.shli
// CHECK: arith.shrui
// LLVM-LABEL: llvm.func @remainder32
// LLVM: llvm.icmp "uge"
// LLVM-LABEL: llvm.func @remainder64
// LLVM: llvm.icmp "uge"
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["float-remainder.mlir"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @remainder32(%x: f32 {sec.scalar_kind = "float32"}, %y: f32 {sec.scalar_kind = "float32"}) -> f32 attributes {sec.function_id = "main::remainder32(float32,float32)", sec.source_name = "remainder32", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x", "y"]} {
    %v = "sec.float.binary"(%x, %y) {kind = "rem"} : (f32, f32) -> f32
    return %v : f32
  }
  func.func @remainder64(%x: f64 {sec.scalar_kind = "float64"}, %y: f64 {sec.scalar_kind = "float64"}) -> f64 attributes {sec.function_id = "main::remainder64(float64,float64)", sec.source_name = "remainder64", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x", "y"]} {
    %v = "sec.float.binary"(%x, %y) {kind = "rem"} : (f64, f64) -> f64
    return %v : f64
  }
}
