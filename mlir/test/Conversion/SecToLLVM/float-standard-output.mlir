// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout --sec-lower-floats | FileCheck %s --check-prefix=ARITH
// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s --check-prefix=LLVM

// P20 §§ 90, 93 standard output on the one native track (schema 16):
// sub/mul are arith.subf/mulf without fast-math; the truncation remainder,
// Abs and Ceiling/Truncate stay Arith-only (bit fields and a |x| < 2^p
// guarded fptosi/sitofp) instead of arith.remf/math.*, so no libm call
// appears; exact and checked conversions lower to fpext, uitofp, fptrunc and
// a guarded fptoui. Deviations: sec_mlir_dialect_v16.md § 18.

module attributes {sec.dialect_version = 16 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%a: f64, %b: f64) -> f64 attributes {sec.function_id = "main::Arith(float64,float64)", sec.source_name = "Arith", sec.extern = false, sec.unsafe = false} {
    %s = "sec.float.binary"(%a, %b) <{kind = "sub"}> : (f64, f64) -> f64
    %m = "sec.float.binary"(%s, %b) <{kind = "mul"}> : (f64, f64) -> f64
    %r = "sec.float.binary"(%m, %a) <{kind = "rem"}> : (f64, f64) -> f64
    %abs = "sec.float.abs"(%r) : (f64) -> f64
    %c = "sec.float.round"(%abs) <{kind = "ceiling"}> : (f64) -> f64
    %t = "sec.float.round"(%c) <{kind = "truncate"}> : (f64) -> f64
    return %t : f64
  }
  func.func @sec_fn_1(%x: f32) -> f64 attributes {sec.function_id = "main::Widen(float32)", sec.source_name = "Widen", sec.extern = false, sec.unsafe = false} {
    %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (f32) -> f64
    return %y : f64
  }
  func.func @sec_fn_2(%x: ui16) -> f32 attributes {sec.function_id = "main::FromU(uint16)", sec.source_name = "FromU", sec.extern = false, sec.unsafe = false} {
    %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (ui16) -> f32
    return %y : f32
  }
  func.func @sec_fn_3(%x: f64) -> (f32, i1, i1, i1) attributes {sec.function_id = "main::Narrow(float64)", sec.source_name = "Narrow", sec.extern = false, sec.unsafe = false} {
    %c, %failed, %range, %precision = "sec.numeric.convert_checked"(%x) <{failures = ["OutOfRange", "PrecisionLoss"]}> : (f64) -> (f32, i1, i1, i1)
    return %c, %failed, %range, %precision : f32, i1, i1, i1
  }
  func.func @sec_fn_4(%x: f64) -> (ui8, i1, i1, i1, i1) attributes {sec.function_id = "main::ToU(float64)", sec.source_name = "ToU", sec.extern = false, sec.unsafe = false} {
    %c, %failed, %nf, %range, %precision = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite", "OutOfRange", "PrecisionLoss"]}> : (f64) -> (ui8, i1, i1, i1, i1)
    return %c, %failed, %nf, %range, %precision : ui8, i1, i1, i1, i1
  }
}

// ARITH-LABEL: func.func @sec_fn_0
// ARITH: arith.subf %{{.*}}, %{{.*}} : f64
// ARITH-NEXT: arith.mulf %{{.*}}, %{{.*}} : f64
// ARITH: arith.bitcast %{{.*}} : f64 to i64
// ARITH: arith.andi %{{.*}}, %{{.*}} : i64
// ARITH: cf.cond_br
// ARITH: arith.fptosi %{{.*}} : f64 to i64
// ARITH-NEXT: arith.sitofp %{{.*}} : i64 to f64
// ARITH: arith.fptosi %{{.*}} : f64 to i64
// ARITH-NEXT: arith.sitofp %{{.*}} : i64 to f64
// ARITH: return %{{.*}} : f64
// ARITH-NOT: arith.remf
// ARITH-NOT: math.
// ARITH-NOT: fastmath

// LLVM-LABEL: llvm.func @sec_fn_0
// LLVM: llvm.fsub
// LLVM: llvm.fmul
// LLVM-LABEL: llvm.func @sec_fn_1
// LLVM: llvm.fpext %{{.*}} : f32 to f64
// LLVM-LABEL: llvm.func @sec_fn_2
// LLVM: llvm.uitofp %{{.*}} : i16 to f32
// LLVM-LABEL: llvm.func @sec_fn_3
// LLVM: llvm.fptrunc %{{.*}} : f64 to f32
// LLVM-LABEL: llvm.func @sec_fn_4
// LLVM: llvm.fcmp "uno"
// LLVM: llvm.fptoui %{{.*}} : f64 to i8
// LLVM-NOT: llvm.frem
// LLVM-NOT: llvm.call
// LLVM-NOT: fastmath
