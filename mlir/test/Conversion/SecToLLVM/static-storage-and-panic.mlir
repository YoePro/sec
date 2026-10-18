// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s

// Static storage becomes internal LLVM globals (constant when immutable) read
// and written through their address; an explicit panic keeps its reason and
// message in Sec MLIR and reaches the minimal panic endpoint (trap).

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "7", is_mutable = false, sym_name = "sec_static_1", type = !sec.int}> {sec.source_name = "Limit"} : () -> ()
  "sec.static.global"() <{initial = "-3", is_mutable = true, sym_name = "sec_static_2", type = si8}> {sec.source_name = "counter"} : () -> ()
  "sec.static.global"() <{initial = "true", is_mutable = true, sym_name = "sec_static_3", type = i1}> {sec.source_name = "armed"} : () -> ()
  func.func @sec_fn_0(%x: si8 {sec.scalar_kind = "int8"}) -> (!sec.int {sec.scalar_kind = "int"}) attributes {sec.function_id = "main::Use(int8)", sec.source_name = "Use", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x"]} {
    "sec.static.store"(%x) <{global = @sec_static_2}> : (si8) -> ()
    %armed = "sec.static.load"() <{global = @sec_static_3}> : () -> i1
    cf.cond_br %armed, ^ok, ^boom
  ^boom:
    "sec.fail.panic"() <{message = "disarmed", reason = "ExplicitPanic"}> : () -> ()
  ^ok:
    %limit = "sec.static.load"() <{global = @sec_static_1}> : () -> !sec.int
    return %limit : !sec.int
  }
}

// CHECK: llvm.mlir.global internal constant @sec_static_1(7 : i64)
// CHECK: llvm.mlir.global internal @sec_static_2(-3 : i8)
// CHECK: llvm.mlir.global internal @sec_static_3(true)
// CHECK: llvm.mlir.addressof @sec_static_2
// CHECK: llvm.store
// CHECK: llvm.mlir.addressof @sec_static_3
// CHECK: llvm.load {{.*}} -> i1
// CHECK: llvm.intr.trap
// CHECK: llvm.mlir.addressof @sec_static_1
// CHECK: llvm.load {{.*}} -> i64
