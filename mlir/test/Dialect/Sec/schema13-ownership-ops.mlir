// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  // CHECK-LABEL: func.func @values
  func.func @values(%v: !sec.int, %p: !sec.place<!sec.int, "rw">, %r: !sec.ref<!sec.int>) {
    // CHECK: sec.own.copy
    %a = "sec.own.copy"(%v) : (!sec.int) -> !sec.int
    // CHECK: sec.own.semantic_copy
    %b = "sec.own.semantic_copy"(%a) <{semantic_copy_plan = "copy#1"}> : (!sec.int) -> !sec.int
    // CHECK: sec.own.move
    %c = "sec.own.move"(%b) : (!sec.int) -> !sec.int
    // CHECK: sec.own.copy_from_place
    %d = "sec.own.copy_from_place"(%p) <{copy_mode = "trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
    // CHECK: sec.own.move_from_place
    %e = "sec.own.move_from_place"(%p) : (!sec.place<!sec.int, "rw">) -> !sec.int
    // CHECK: sec.own.initialize_place
    "sec.own.initialize_place"(%p, %e) : (!sec.place<!sec.int, "rw">, !sec.int) -> ()
    // CHECK: sec.own.replace_place
    "sec.own.replace_place"(%p, %c) <{destruction_plan = "plan#1", old_cleanup_action_id = 1 : i32, new_cleanup_action_id = 2 : i32}> : (!sec.place<!sec.int, "rw">, !sec.int) -> ()
    // CHECK: sec.own.discard
    "sec.own.discard"(%d) <{discard_origin = "explicit", destruction_plan = "plan#1"}> : (!sec.int) -> ()
    // CHECK: sec.own.discard
    "sec.own.discard"(%r) <{discard_origin = "compiler-temporary"}> : (!sec.ref<!sec.int>) -> ()
    // CHECK: sec.own.destroy_value
    "sec.own.destroy_value"(%a) <{destruction_plan = "plan#1", cause = "scope-exit"}> : (!sec.int) -> ()
    // CHECK: sec.own.destroy_place
    "sec.own.destroy_place"(%p) <{destruction_plan = "plan#1", cause = "function-exit", skipped = ["field#0"]}> : (!sec.place<!sec.int, "rw">) -> ()
    return
  }

  // CHECK-LABEL: func.func @cleanups
  func.func @cleanups(%v: !sec.int, %p: !sec.place<!sec.int, "rw">) {
    // CHECK: sec.cleanup.track_owned
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "plan#1"}> : (!sec.int) -> ()
    // CHECK: sec.cleanup.defer_register
    "sec.cleanup.defer_register"(%p) <{cleanup_action_id = 2 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, defer_id = 1 : i32}> ({
    ^bb0(%capture: !sec.place<!sec.int, "rw">):
      %read = "sec.own.copy_from_place"(%capture) <{copy_mode = "trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
      // CHECK: sec.cleanup.defer_yield
      "sec.cleanup.defer_yield"() : () -> ()
    }) : (!sec.place<!sec.int, "rw">) -> ()
    // CHECK: sec.cleanup.cancel
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    // CHECK: sec.cleanup.cancel
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "discarded"}> : () -> ()
    // CHECK: sec.cleanup.run_scope
    "sec.cleanup.run_scope"() <{scope_ids = array<i32: 2, 1>, exit_kind = "break"}> : () -> ()
    // CHECK: sec.cleanup.run_function
    "sec.cleanup.run_function"() <{exit_kind = "error-propagation"}> : () -> ()
    return
  }
}
