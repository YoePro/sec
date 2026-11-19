// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s

// One module's object (linking.md §§ 9, 15, 23-24): an exported definition
// keeps external linkage, a private definition (a specialization every
// demanding module instantiates) becomes internal, and an imported Sec
// declaration stays an external declaration.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "util", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["util/util.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_sym_answer(%x: i64) -> i64 attributes {sec.function_id = "util::Answer(int64)", sec.source_name = "Answer", sec.extern = false, sec.unsafe = false} {
    %v = "sec.call.direct"(%x) <{callee = @sec_overload_id}> {sec.argument_actions = ["copy-trivial"]} : (i64) -> i64
    %w = "sec.call.direct"(%v) <{callee = @sec_sym_other}> {sec.argument_actions = ["copy-trivial"]} : (i64) -> i64
    return %w : i64
  }
  func.func private @sec_overload_id(%x: i64) -> i64 attributes {sec.function_id = "util::Id[T=int64](int64)", sec.source_name = "Id", sec.extern = false, sec.unsafe = false} {
    return %x : i64
  }
  func.func private @sec_sym_other(i64) -> i64 attributes {sec.function_id = "other::F(int64)", sec.source_name = "other.F", sec.extern = false, sec.unsafe = false, sec.imported = true}
}

// CHECK-DAG: llvm.func @sec_sym_answer(
// CHECK-DAG: llvm.func internal @sec_overload_id(
// CHECK-DAG: llvm.func @sec_sym_other(i64) -> i64
