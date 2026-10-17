// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s

// A loaded local keeps its source signedness for checked comparison and
// arithmetic in a loop; storage normalization leaves no cast behind.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%limit: !sec.int {sec.scalar_kind = "int"}) -> (!sec.int {sec.scalar_kind = "int"}) attributes {sec.function_id = "main::Count(int)", sec.source_name = "Count", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["limit"]} {
    %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "i", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.scalar_kind = "int"} : () -> !sec.storage<!sec.int>
    "sec.storage.init"(%slot, %zero) : (!sec.storage<!sec.int>, !sec.int) -> ()
    cf.br ^header
  ^header:
    %i = "sec.storage.load"(%slot) : (!sec.storage<!sec.int>) -> !sec.int
    %continue = "sec.int.cmp"(%i, %limit) <{predicate = "lt"}> : (!sec.int, !sec.int) -> i1
    cf.cond_br %continue, ^body, ^exit
  ^body:
    %current = "sec.storage.load"(%slot) : (!sec.storage<!sec.int>) -> !sec.int
    %one = "sec.const.int"() <{value = "1"}> : () -> !sec.int
    %next, %failed, %reason = "sec.int.binary_checked"(%current, %one) <{kind = "add"}> : (!sec.int, !sec.int) -> (!sec.int, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %failed, ^fail(%reason : !sec.arithmetic_failure_reason), ^store
  ^exit:
    %result = "sec.storage.load"(%slot) : (!sec.storage<!sec.int>) -> !sec.int
    return %result : !sec.int
  ^fail(%r: !sec.arithmetic_failure_reason):
    "sec.fail.arithmetic"(%r) {sec.operator = "+"} : (!sec.arithmetic_failure_reason) -> ()
  ^store:
    "sec.storage.store"(%slot, %next) : (!sec.storage<!sec.int>, !sec.int) -> ()
    cf.br ^header
  }
}

// CHECK: llvm.func @sec_fn_0(%{{.*}}: i64) -> i64
// CHECK: llvm.alloca
// CHECK: llvm.load {{.*}} -> i64
// CHECK: llvm.icmp "slt"
// CHECK: llvm.intr.trap
// CHECK-NOT: unrealized_conversion_cast
// CHECK-NOT: sec.
