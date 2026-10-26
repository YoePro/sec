// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s
// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-lower-floats | FileCheck %s --check-prefix=ARITH

// P20 core floating members (§§ 28–36, 78, 80): Abs clears the sign bit,
// Min/Max are NaN-propagating minimumf/maximumf (never minnumf/maxnumf),
// classification uses the binary field masks, rounding needs no runtime
// library, and properties are exact constants.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%a: f64 {sec.scalar_kind = "float64"}, %b: f32 {sec.scalar_kind = "float32"}) -> i1 attributes {sec.function_id = "main::F(float64,float32)", sec.source_name = "F", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["a", "b"]} {
    %abs = "sec.float.abs"(%a) : (f64) -> f64
    %max = "sec.float.property"() <{kind = "max"}> : () -> f64
    %min = "sec.float.minimum"(%abs, %max) : (f64, f64) -> f64
    %top = "sec.float.maximum"(%min, %a) : (f64, f64) -> f64
    %round = "sec.float.round"(%top) <{kind = "nearest-even"}> : (f64) -> f64
    %nan = "sec.float.classify"(%round) <{kind = "nan"}> : (f64) -> i1
    %floor = "sec.float.round"(%b) <{kind = "floor"}> : (f32) -> f32
    %sub = "sec.float.classify"(%floor) <{kind = "subnormal"}> : (f32) -> i1
    %eps = "sec.float.property"() <{kind = "epsilon"}> : () -> f32
    %quiet = "sec.float.property"() <{kind = "nan"}> : () -> f32
    %either = "sec.float.cmp"(%eps, %quiet) <{predicate = "ne"}> : (f32, f32) -> i1
    return %either : i1
  }
}

// ARITH-LABEL: func.func @sec_fn_0
// ARITH: arith.bitcast %{{.*}} : f64 to i64
// ARITH: arith.andi %{{.*}}, %{{.*}} : i64
// ARITH: arith.constant 1.7976931348623157E+308 : f64
// ARITH: arith.minimumf %{{.*}}, %{{.*}} : f64
// ARITH: arith.maximumf %{{.*}}, %{{.*}} : f64
// ARITH: arith.cmpf olt, %{{.*}}, %{{.*}} : f64
// ARITH: arith.fptosi %{{.*}} : f64 to i64
// ARITH: arith.sitofp %{{.*}} : i64 to f64
// ARITH: arith.constant 9218868437227405312 : i64
// ARITH: arith.constant 8388607 : i32
// ARITH: arith.constant 1.1920929E-7 : f32
// ARITH: arith.constant 0x7FC00000 : f32
// ARITH-NOT: minnumf
// ARITH-NOT: maxnumf
// ARITH-NOT: math.
// ARITH-NOT: fastmath

// CHECK-LABEL: llvm.func @sec_fn_0(%{{.*}}: f64, %{{.*}}: f32) -> i1
// CHECK: llvm.intr.minimum
// CHECK: llvm.intr.maximum
// CHECK: llvm.fptosi
// CHECK-NOT: llvm.call
