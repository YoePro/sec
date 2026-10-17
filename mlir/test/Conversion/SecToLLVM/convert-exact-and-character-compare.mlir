// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s

// An exact integer conversion is a width change chosen by the source
// signedness (MD-104); rune and char compare unsigned by code value after
// scalar layout resolves them to ui32/ui8 (MD-105).

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%a: ui8 {sec.scalar_kind = "uint8"}, %b: si8 {sec.scalar_kind = "int8"}) -> (si64 {sec.scalar_kind = "int64"}) attributes {sec.function_id = "main::Widen(uint8,int8)", sec.source_name = "Widen", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["a", "b"]} {
    %wa = "sec.numeric.convert_exact"(%a) : (ui8) -> si64
    %wb = "sec.numeric.convert_exact"(%b) : (si8) -> si64
    %sum, %failed, %reason = "sec.int.binary_checked"(%wa, %wb) <{kind = "add"}> : (si64, si64) -> (si64, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %failed, ^fail(%reason : !sec.arithmetic_failure_reason), ^ok
  ^fail(%r: !sec.arithmetic_failure_reason):
    "sec.fail.arithmetic"(%r) {sec.operator = "+"} : (!sec.arithmetic_failure_reason) -> ()
  ^ok:
    return %sum : si64
  }
  func.func @sec_fn_1(%x: si64 {sec.scalar_kind = "int64"}) -> (si8 {sec.scalar_kind = "int8"}) attributes {sec.function_id = "main::Narrow(int64)", sec.source_name = "Narrow", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x"]} {
    %n = "sec.numeric.convert_exact"(%x) : (si64) -> si8
    return %n : si8
  }
  func.func @sec_fn_2(%r: !sec.rune {sec.scalar_kind = "rune"}, %s: !sec.rune {sec.scalar_kind = "rune"}) -> i1 attributes {sec.function_id = "main::Before(rune,rune)", sec.source_name = "Before", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["r", "s"]} {
    %lt = "sec.int.cmp"(%r, %s) <{predicate = "lt"}> : (!sec.rune, !sec.rune) -> i1
    return %lt : i1
  }
}

// CHECK-LABEL: llvm.func @sec_fn_0(%{{.*}}: i8, %{{.*}}: i8) -> i64
// CHECK: llvm.zext %{{.*}} : i8 to i64
// CHECK: llvm.sext %{{.*}} : i8 to i64
// CHECK-LABEL: llvm.func @sec_fn_1(%{{.*}}: i64) -> i8
// CHECK: llvm.trunc %{{.*}} : i64 to i8
// CHECK-LABEL: llvm.func @sec_fn_2(%{{.*}}: i32, %{{.*}}: i32) -> i1
// CHECK: llvm.icmp "ult"
