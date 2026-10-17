// RUN: sec-mlir-opt %s --split-input-file --sec-resolve-scalar-layout --sec-lower-integer-enums | FileCheck %s --check-prefix=ERASE
// RUN: sec-mlir-opt %s --split-input-file --sec-lower-to-llvm=entry= | FileCheck %s --check-prefix=LLVM
// RUN: sec-mlir-opt %s --split-input-file --sec-resolve-scalar-layout | FileCheck %s --check-prefix=KEEP

// The P6 boundary preserves nominal enums. Only the dedicated representation
// stage erases them. Target-sized carriers follow each module's plan; constants
// use case values (not ordinals), including aliases and signed extremes.
!State = !sec.enum<identity = "main::State", underlying = !sec.int, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "first", value = "17">, #sec.enum_case<ordinal = 1, name = "second", value = "43">, #sec.enum_case<ordinal = 2, name = "alias", value = "43">]>
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.source_files = ["main.sec"], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @choose(%condition: i1) -> !State {
    cf.cond_br %condition, ^first, ^second
  ^first:
    %a = "sec.enum.constant"() <{caseOrdinal = 0 : i32}> : () -> !State
    cf.br ^join(%a : !State)
  ^second:
    %b = "sec.enum.constant"() <{caseOrdinal = 2 : i32}> : () -> !State
    cf.br ^join(%b : !State)
  ^join(%value: !State):
    return %value : !State
  }
}
// ERASE: func.func @choose(%{{.*}}: i1) -> si32
// ERASE: "sec.const.int"() <{value = "17"}> : () -> si32
// ERASE: "sec.const.int"() <{value = "43"}> : () -> si32
// ERASE: ^{{.*}}(%{{.*}}: si32)
// LLVM: llvm.func @choose(%{{.*}}: i1) -> i32
// LLVM: llvm.mlir.constant(17 : i32)
// LLVM: llvm.mlir.constant(43 : i32)
// KEEP: !sec.enum<identity = "main::State", underlying = si32

// -----

!State = !sec.enum<identity = "main::State", underlying = !sec.int, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "low", value = "-9223372036854775808">, #sec.enum_case<ordinal = 1, name = "high", value = "9223372036854775807">]>
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.source_files = ["main.sec"], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @equal(%other: !State) -> i1 {
    %low = "sec.enum.constant"() <{caseOrdinal = 0 : i32}> : () -> !State
    %equal = "sec.enum.cmp"(%low, %other) <{predicate = "eq"}> : (!State, !State) -> i1
    return %equal : i1
  }
}
// ERASE: func.func @equal(%{{.*}}: si64) -> i1
// ERASE: "sec.const.int"() <{value = "-9223372036854775808"}> : () -> si64
// ERASE: "sec.int.cmp"
// LLVM: llvm.func @equal(%{{.*}}: i64) -> i1
// LLVM: llvm.mlir.constant(-9223372036854775808 : i64)
// LLVM: llvm.icmp "eq"
// KEEP: !sec.enum<identity = "main::State", underlying = si64
