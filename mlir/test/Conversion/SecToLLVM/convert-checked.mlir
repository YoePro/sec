// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s

// A checked integer conversion (P20 §§ 48–54, 71): the source, extended one
// bit beyond both widths by its own signedness, is compared with the target
// domain; the candidate is the width change and is used only on success.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%x: si32 {sec.scalar_kind = "int32"}) -> (si8 {sec.scalar_kind = "int8"}) attributes {sec.function_id = "main::Narrow(int32)", sec.source_name = "Narrow", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x"]} {
    %candidate, %failed = "sec.numeric.convert_checked"(%x) <{failure = "OutOfRange"}> : (si32) -> (si8, i1)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    %fallback = "sec.const.int"() <{value = "-1"}> : () -> si8
    return %fallback : si8
  ^ok:
    return %candidate : si8
  }
  func.func @sec_fn_1(%x: si64 {sec.scalar_kind = "int64"}) -> (ui64 {sec.scalar_kind = "uint64"}) attributes {sec.function_id = "main::Unsigned(int64)", sec.source_name = "Unsigned", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x"]} {
    %candidate, %failed = "sec.numeric.convert_checked"(%x) <{failure = "OutOfRange"}> : (si64) -> (ui64, i1)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    %zero = "sec.const.int"() <{value = "0"}> : () -> ui64
    return %zero : ui64
  ^ok:
    return %candidate : ui64
  }
}

// CHECK-LABEL: llvm.func @sec_fn_0(%{{.*}}: i32) -> i8
// CHECK: llvm.sext %{{.*}} : i32 to i33
// CHECK: llvm.mlir.constant(-128 : i33)
// CHECK: llvm.icmp "slt"
// CHECK: llvm.mlir.constant(127 : i33)
// CHECK: llvm.icmp "sgt"
// CHECK: llvm.or
// CHECK: llvm.trunc %{{.*}} : i32 to i8
// CHECK-LABEL: llvm.func @sec_fn_1(%{{.*}}: i64) -> i64
// CHECK: llvm.sext %{{.*}} : i64 to i65
// CHECK: llvm.mlir.constant(0 : i65)
// CHECK: llvm.icmp "slt"
// CHECK: llvm.mlir.constant(18446744073709551615 : i65)
// CHECK: llvm.icmp "sgt"
