// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s
// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-lower-floats | FileCheck %s --check-prefix=ARITH

// P20 binary floating subset: constants lower from their canonical bit
// pattern (0.1 as binary32 is 0x3dcccccd = 1036831949), arithmetic and
// comparisons lower to arith without fast-math, `!=` is unordered and the
// rest ordered, `rem` is the truncation remainder, and plain float takes the
// platform width (MD-014).

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%a: f32 {sec.scalar_kind = "float32"}, %b: !sec.float {sec.scalar_kind = "float"}) -> i1 attributes {sec.function_id = "main::F(float32,float)", sec.source_name = "F", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["a", "b"]} {
    %tenth = "sec.const.float"() <{bits = 1036831949 : i64, lexeme = "0.1"}> : () -> f32
    %sum = "sec.float.binary"(%a, %tenth) <{kind = "add"}> : (f32, f32) -> f32
    %rem = "sec.float.binary"(%sum, %a) <{kind = "rem"}> : (f32, f32) -> f32
    %neg = "sec.float.neg"(%rem) : (f32) -> f32
    %ne = "sec.float.cmp"(%neg, %a) <{predicate = "ne"}> : (f32, f32) -> i1
    %two = "sec.const.float"() <{bits = 4611686018427387904 : i64, lexeme = "2.0"}> : () -> !sec.float
    %half = "sec.float.binary"(%b, %two) <{kind = "div"}> : (!sec.float, !sec.float) -> !sec.float
    %lt = "sec.float.cmp"(%half, %b) <{predicate = "lt"}> : (!sec.float, !sec.float) -> i1
    return %lt : i1
  }
}

// ARITH-LABEL: func.func @sec_fn_0(%{{.*}}: f32 {{.*}}, %{{.*}}: f64 {{.*}}) -> i1
// ARITH: arith.constant 1.000000e-01 : f32
// ARITH: arith.addf %{{.*}}, %{{.*}} : f32
// ARITH: arith.cmpi uge
// ARITH: cf.cond_br
// ARITH: arith.negf %{{.*}} : f32
// ARITH: arith.cmpf une, %{{.*}}, %{{.*}} : f32
// ARITH: arith.constant 2.000000e+00 : f64
// ARITH: arith.divf %{{.*}}, %{{.*}} : f64
// ARITH: arith.cmpf olt, %{{.*}}, %{{.*}} : f64
// ARITH-NOT: fastmath

// CHECK-LABEL: llvm.func @sec_fn_0(%{{.*}}: f32, %{{.*}}: f64) -> i1
// CHECK: llvm.fadd
// CHECK: llvm.icmp "uge"
// CHECK: llvm.fneg
// CHECK: llvm.fcmp "une"
// CHECK: llvm.fdiv
// CHECK: llvm.fcmp "olt"
