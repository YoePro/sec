// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s

// P20 conversion lowering (sec-mlir-dialect_package20.md §§ 47-55, 79):
// float-to-bool is an unordered compare with +0.0; a checked float-to-int
// conversion guards non-finite, range and integrality and only ever converts
// a sanitized operand.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%x: f64) -> i1 attributes {sec.function_id = "main::Truth(float64)", sec.source_name = "Truth", sec.extern = false, sec.unsafe = false} {
    %y = "sec.float.to_bool"(%x) : (f64) -> i1
    return %y : i1
  }
  func.func @sec_fn_1(%x: f64) -> (si32, i1, i1, i1, i1) attributes {sec.function_id = "main::Checked(float64)", sec.source_name = "Checked", sec.extern = false, sec.unsafe = false} {
    %c, %failed, %nf, %range, %precision = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite", "OutOfRange", "PrecisionLoss"]}> : (f64) -> (si32, i1, i1, i1, i1)
    return %c, %failed, %nf, %range, %precision : si32, i1, i1, i1, i1
  }
}

// CHECK-LABEL: llvm.func @sec_fn_0
// CHECK: llvm.fcmp "une"
// CHECK-LABEL: llvm.func @sec_fn_1
// CHECK: llvm.fcmp "uno"
// CHECK: llvm.select
// CHECK: llvm.fptosi {{.*}} : f64 to i64
// CHECK: llvm.select
// CHECK: llvm.fptosi {{.*}} : f64 to i32
