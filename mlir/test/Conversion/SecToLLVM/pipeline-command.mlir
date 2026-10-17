// RUN: sec-mlir-opt %s --sec-lower-to-llvm | FileCheck %s
// RUN: sec-mlir-opt %s --sec-lower-to-llvm | mlir-translate --mlir-to-llvmir | FileCheck %s --check-prefix=LLVMIR

// The basic Sec MLIR -> LLVM path: checked arithmetic keeps its failure edge,
// the failure terminator becomes the minimal panic endpoint, and the hosted
// startup adapter returns the command status as the target C int.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%a: !sec.int {sec.scalar_kind = "int"}, %b: !sec.int {sec.scalar_kind = "int"}) -> (!sec.int {sec.scalar_kind = "int"}) attributes {sec.function_id = "main::Add(int,int)", sec.source_name = "Add", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["a", "b"]} {
    %sum, %failed, %reason = "sec.int.binary_checked"(%a, %b) <{kind = "add"}> : (!sec.int, !sec.int) -> (!sec.int, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %failed, ^fail(%reason : !sec.arithmetic_failure_reason), ^ok
  ^fail(%r: !sec.arithmetic_failure_reason):
    "sec.fail.arithmetic"(%r) {sec.operator = "+"} : (!sec.arithmetic_failure_reason) -> ()
  ^ok:
    return %sum : !sec.int
  }
  func.func @sec_fn_1() -> (!sec.int {sec.scalar_kind = "int"}) attributes {sec.function_id = "main::main()", sec.source_name = "main", sec.extern = false, sec.unsafe = false} {
    %two = "sec.const.int"() <{value = "2"}> : () -> !sec.int
    %three = "sec.const.int"() <{value = "3"}> : () -> !sec.int
    %r = "sec.call.direct"(%two, %three) <{callee = @sec_fn_0}> {sec.argument_actions = ["copy-trivial", "copy-trivial"]} : (!sec.int, !sec.int) -> !sec.int
    return %r : !sec.int
  }
}

// CHECK: module attributes
// CHECK-SAME: llvm.target_triple = "x86_64-pc-linux-gnu"
// CHECK-NOT: sec.
// CHECK: llvm.func @sec_fn_0(%{{.*}}: i64, %{{.*}}: i64) -> i64
// CHECK: llvm.add {{.*}} : i65
// CHECK: llvm.cond_br
// CHECK: llvm.intr.trap
// CHECK-NEXT: llvm.unreachable
// CHECK: llvm.func @main() -> i32
// CHECK: llvm.call @sec_fn_1() : () -> i64
// CHECK: llvm.trunc {{.*}} : i64 to i32

// LLVMIR: target triple = "x86_64-pc-linux-gnu"
// LLVMIR: call void @llvm.trap()
// LLVMIR: define i32 @main()
