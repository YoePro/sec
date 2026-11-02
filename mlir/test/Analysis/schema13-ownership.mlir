// RUN: sec-mlir-opt %s --sec-verify-ownership | FileCheck %s
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 9-18, 111-114.

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  // CHECK-LABEL: func.func @exclusive
  func.func @exclusive(%c: i1) {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    cf.cond_br %c, ^bb1, ^bb2
    ^bb1:
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^bb2:
    "sec.own.discard"(%v) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
  // CHECK-LABEL: func.func @merged
  func.func @merged(%c: i1) {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    cf.cond_br %c, ^bb1(%v : !h), ^bb2(%v : !h)
    ^bb1(%x: !h):
    "sec.call.direct"(%x) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^bb2(%y: !h):
    "sec.own.discard"(%y) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
  // CHECK-LABEL: func.func @joined
  func.func @joined(%c: i1) {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    cf.cond_br %c, ^bb1, ^bb2
    ^bb1:
    %w = "sec.own.move"(%v) : (!h) -> !h
    cf.br ^bb3(%w : !h)
    ^bb2:
    cf.br ^bb3(%v : !h)
    ^bb3(%m: !h):
    "sec.call.direct"(%m) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
  // CHECK-LABEL: func.func @reinit
  func.func @reinit() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!h>
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    "sec.own.initialize_place"(%p0, %v) : (!sec.place<!h, "rw">, !h) -> ()
    %p1 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    %o = "sec.own.move_from_place"(%p1) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    %n = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    %p2 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    "sec.own.initialize_place"(%p2, %n) : (!sec.place<!h, "rw">, !h) -> ()
    %p3 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    "sec.own.destroy_place"(%p3) <{destruction_plan = "h", cause = "function-exit"}> : (!sec.place<!h, "rw">) -> ()
    return
  }
  // CHECK-LABEL: func.func @partial
  func.func @partial(%a: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p0, %a) : (!sec.place<!p, "rw">, !p) -> ()
    %f = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %o = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    %p1 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.destroy_place"(%p1) <{destruction_plan = "p", cause = "function-exit", skipped = ["field#2"]}> : (!sec.place<!p, "rw">) -> ()
    return
  }
  // CHECK-LABEL: func.func @partial_reinit
  func.func @partial_reinit(%a: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p0, %a) : (!sec.place<!p, "rw">, !p) -> ()
    %f = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %o = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    %g = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    "sec.own.initialize_place"(%g, %o) : (!sec.place<!h, "rw">, !h) -> ()
    "sec.own.destroy_place"(%p0) <{destruction_plan = "p", cause = "function-exit"}> : (!sec.place<!p, "rw">) -> ()
    return
  }
  // CHECK-LABEL: func.func @result
  func.func @result(%r: !e) {
    %b = "sec.result.is_err"(%r) : (!e) -> i1
    cf.cond_br %b, ^bb1, ^bb2
    ^bb1:
    %x = "sec.result.unwrap_err"(%r) : (!e) -> !sec.int
    return
    ^bb2:
    %o = "sec.result.unwrap_ok"(%r) : (!e) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
  // CHECK-LABEL: func.func @looped
  func.func @looped(%c: i1) {
    cf.br ^bb1
    ^bb1:
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    cf.cond_br %c, ^bb1, ^bb2
    ^bb2:
    return
  }
  // CHECK-LABEL: func.func @scoped
  func.func @scoped(%c: i1) {
    cf.br ^bb1
    ^bb1:
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "h"}> : (!h) -> ()
    "sec.cleanup.run_scope"() <{scope_ids = array<i32: 1>, exit_kind = "continue"}> : () -> ()
    cf.cond_br %c, ^bb1, ^bb2
    ^bb2:
    return
  }
  // CHECK-LABEL: func.func @copyable
  func.func @copyable(%i: !sec.int) {
    %c = "sec.own.copy"(%i) : (!sec.int) -> !sec.int
    %d = "sec.own.copy"(%i) : (!sec.int) -> !sec.int
    return
  }
}
