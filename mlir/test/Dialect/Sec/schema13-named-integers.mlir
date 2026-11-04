// RUN: sec-mlir-opt %s | FileCheck %s
// Named integers (dialect v11.41.7): integer operations accept a named integer
// and named.convert changes only the nominal identity.

!m = !sec.named<"main::Meters", !sec.int>
!k = !sec.named<"main::Kilometers", !m>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "named", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // CHECK-LABEL: func.func @named
  func.func @named(%raw: !sec.int, %other: !m) -> i1 {
    // CHECK: sec.named.convert
    %meters = "sec.named.convert"(%raw) : (!sec.int) -> !m
    %km = "sec.named.convert"(%meters) : (!m) -> !k
    %back = "sec.named.convert"(%km) : (!k) -> !m
    %plus = "sec.int.unary_plus"(%back) : (!m) -> !m
    %less = "sec.int.cmp"(%plus, %other) <{predicate = "lt"}> : (!m, !m) -> i1
    return %less : i1
  }
}
