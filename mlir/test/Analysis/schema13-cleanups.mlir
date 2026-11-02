// RUN: sec-mlir-opt %s --sec-verify-cleanups | FileCheck %s

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  // CHECK-LABEL: func.func @conditional
  func.func @conditional(%v: !sec.int, %c: i1) -> !sec.int {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    cf.cond_br %c, ^bb1, ^bb2
  ^bb1:
    %m = "sec.own.move"(%v) : (!sec.int) -> !sec.int
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "transferred"}> : () -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return %m : !sec.int
  ^bb2:
    %c0 = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return %c0 : !sec.int
  }
  // CHECK-LABEL: func.func @scoped
  func.func @scoped(%v: !sec.int, %c: i1) {
    cf.br ^bb1
  ^bb1:
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 2 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    "sec.cleanup.defer_register"() <{cleanup_action_id = 2 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, defer_id = 2 : i32}> ({
      "sec.cleanup.defer_yield"() : () -> ()
    }) : () -> ()
    "sec.cleanup.run_scope"() <{scope_ids = array<i32: 2>, exit_kind = "continue"}> : () -> ()
    cf.cond_br %c, ^bb1, ^bb2
  ^bb2:
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
  // CHECK-LABEL: func.func @panics
  func.func @panics(%v: !sec.int, %c: i1) {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    cf.cond_br %c, ^bb1, ^bb2
  ^bb1:
    "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
  ^bb2:
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}
