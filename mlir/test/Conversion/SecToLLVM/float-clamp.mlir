// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-lower-floats | FileCheck %s

// P20 §§ 31, 69: Clamp fails for a NaN bound (unordered) or minimum >
// maximum; otherwise the value is clamped with the NaN-propagating
// minimumf/maximumf, so a NaN value stays NaN.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%x: f64 {sec.scalar_kind = "float64"}, %lo: f64 {sec.scalar_kind = "float64"}, %hi: f64 {sec.scalar_kind = "float64"}) -> f64 attributes {sec.function_id = "main::C(float64,float64,float64)", sec.source_name = "C", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x", "lo", "hi"]} {
    %v, %failed = "sec.float.clamp_checked"(%x, %lo, %hi) : (f64, f64, f64) -> (f64, i1)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    return %lo : f64
  ^ok:
    return %v : f64
  }
}

// CHECK-LABEL: func.func @sec_fn_0
// CHECK: %[[UNO:.*]] = arith.cmpf uno, %{{.*}}, %{{.*}} : f64
// CHECK: %[[GT:.*]] = arith.cmpf ogt, %{{.*}}, %{{.*}} : f64
// CHECK: %[[FAILED:.*]] = arith.ori %[[UNO]], %[[GT]] : i1
// CHECK: %[[UPPER:.*]] = arith.minimumf %{{.*}}, %{{.*}} : f64
// CHECK: arith.maximumf %[[UPPER]], %{{.*}} : f64
// CHECK: cf.cond_br %[[FAILED]]
