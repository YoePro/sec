// RUN: sec-mlir-opt %s --sec-verify-ownership --sec-verify-cleanups --sec-verify-destruction-plans | FileCheck %s
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 14, 25-26, 63.

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "r", type = !h, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}, {id = "p", type = !p, kind = "struct", fields = [{field = 1 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makePair() -> !p attributes {sec.function_id = "makePair", sec.source_name = "makePair", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  // CHECK-LABEL: func.func @constructs_directly
  func.func @constructs_directly() -> !o {
    %h = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    %o = "sec.union.construct"(%h) <{fieldNames = [], payloadActions = ["construct-direct"], variant = 0 : i32}> : (!h) -> !o
    "func.return"(%o) {sec.return_action = "move"} : (!o) -> ()
  }
  // CHECK-LABEL: func.func @fresh_projection_takes_the_container
  func.func @fresh_projection_takes_the_container() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !o
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^none
    ^some:
    %h = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^none:
    "sec.own.destroy_value"(%o) <{destruction_plan = "o", cause = "temporary-end"}> : (!o) -> ()
    return
  }
  // CHECK-LABEL: func.func @local_owner_ends_on_the_moving_path
  func.func @local_owner_ends_on_the_moving_path() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !o
    "sec.cleanup.track_owned"(%o) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "o"}> : (!o) -> ()
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^join
    ^some:
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    %h = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    cf.br ^join
    ^join:
    "sec.cleanup.run_function"() <{exit_kind = "fallthrough"}> : () -> ()
    return
  }
  // A mutable owner is observed for its test and its payload leaves through the Place.
  // CHECK-LABEL: func.func @stored_payload_moves_through_the_place
  func.func @stored_payload_moves_through_the_place() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !o
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "o", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!o>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!o>) -> !sec.place<!o, "rw">
    "sec.own.initialize_place"(%p, %o) : (!sec.place<!o, "rw">, !o) -> ()
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "o"}> : (!sec.place<!o, "rw">) -> ()
    %v = "sec.place.read"(%p) <{action = "observe"}> : (!sec.place<!o, "rw">) -> !o
    %c = "sec.union.is_variant"(%v) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^join
    ^some:
    %q = "sec.place.union_payload"(%p) <{variant = 0 : i32, variant_proven = true}> : (!sec.place<!o, "rw">) -> !sec.place<!h, "rw">
    %h = "sec.own.move_from_place"(%q) : (!sec.place<!h, "rw">) -> !h
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    cf.br ^join
    ^join:
    "sec.cleanup.run_function"() <{exit_kind = "fallthrough"}> : () -> ()
    return
  }
  // A false guard leaves the fresh owner whole for the next arm.
  // CHECK-LABEL: func.func @guard_inspects_a_candidate
  func.func @guard_inspects_a_candidate() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !o
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^matched, ^rest
    ^matched:
    %p = "sec.place.value"(%o) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!o) -> !sec.place<!o, "ro">
    %q = "sec.place.union_payload"(%p) <{variant = 0 : i32, variant_proven = true}> : (!sec.place<!o, "ro">) -> !sec.place<!h, "ro">
    %v = "sec.place.read"(%q) <{action = "candidate"}> : (!sec.place<!h, "ro">) -> !h
    %n = "sec.struct.extract"(%v) <{action = "copy-trivial", field = 0 : i32}> : (!h) -> !sec.int
    %z = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    %g = "sec.int.cmp"(%n, %z) <{predicate = "gt"}> : (!sec.int, !sec.int) -> i1
    cf.cond_br %g, ^body, ^rest
    ^body:
    %h = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^rest:
    "sec.own.destroy_value"(%o) <{destruction_plan = "o", cause = "temporary-end"}> : (!o) -> ()
    return
  }
  // A Place moved on some paths is replaced or destroyed only where still owned.
  // CHECK-LABEL: func.func @conditional_replace
  func.func @conditional_replace(%c: i1) {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !o
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "o", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!o>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!o>) -> !sec.place<!o, "rw">
    "sec.own.initialize_place"(%p, %o) : (!sec.place<!o, "rw">, !o) -> ()
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "o"}> : (!sec.place<!o, "rw">) -> ()
    cf.cond_br %c, ^moved, ^join
    ^moved:
    %m = "sec.own.move_from_place"(%p) : (!sec.place<!o, "rw">) -> !o
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    "sec.call.direct"(%m) <{callee = @takeOption}> {sec.argument_actions = ["move"]} : (!o) -> ()
    cf.br ^join
    ^join:
    %n = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !o
    "sec.own.replace_place"(%p, %n) <{destruction_plan = "o", if_owned, old_cleanup_action_id = 1 : i32, new_cleanup_action_id = 2 : i32}> : (!sec.place<!o, "rw">, !o) -> ()
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "replaced", if_active}> : () -> ()
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 2 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, destruction_plan = "o"}> : (!sec.place<!o, "rw">) -> ()
    "sec.cleanup.run_function"() <{exit_kind = "fallthrough"}> : () -> ()
    return
  }
  // A moved stored field leaves its owner registered; a field replacement keeps it.
  // CHECK-LABEL: func.func @field_move_keeps_owner
  func.func @field_move_keeps_owner() {
    %v = "sec.call.direct"() <{callee = @makePair}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !p
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "p", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p, %v) : (!sec.place<!p, "rw">, !p) -> ()
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.place<!p, "rw">) -> ()
    %f = "sec.place.field"(%p) <{field = 0 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %l = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    "sec.call.direct"(%l) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    %n = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    "sec.own.initialize_place"(%f, %n) : (!sec.place<!h, "rw">, !h) -> ()
    %g = "sec.place.field"(%p) <{field = 1 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %m = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    "sec.own.replace_place"(%g, %m) <{destruction_plan = "h", old_cleanup_action_id = 0 : i32, new_cleanup_action_id = 0 : i32}> : (!sec.place<!h, "rw">, !h) -> ()
    "sec.cleanup.run_function"() <{exit_kind = "fallthrough"}> : () -> ()
    return
  }
  // A payload that needs no destruction was all an unregistered fresh owner held.
  // CHECK-LABEL: func.func @trivial_projection_ends_a_fresh_owner
  func.func @trivial_projection_ends_a_fresh_owner() -> !sec.int {
    %r = "sec.call.direct"() <{callee = @makeFailure}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !f
    %c = "sec.result.is_err"(%r) : (!f) -> i1
    cf.cond_br %c, ^err, ^ok
    ^err:
    %h = "sec.result.unwrap_err"(%r) : (!f) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    %z = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    return %z : !sec.int
    ^ok:
    %n = "sec.result.unwrap_ok"(%r) : (!f) -> !sec.int
    return %n : !sec.int
  }
  // The registration still destroys a container whose trivial payload was only read.
  // CHECK-LABEL: func.func @trivial_projection_reads_a_registered_owner
  func.func @trivial_projection_reads_a_registered_owner() -> !sec.int {
    %r = "sec.call.direct"() <{callee = @makeFailure}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !f
    "sec.cleanup.track_owned"(%r) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "f"}> : (!f) -> ()
    %c = "sec.result.is_err"(%r) : (!f) -> i1
    cf.cond_br %c, ^err, ^ok
    ^err:
    %z = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return %z : !sec.int
    ^ok:
    %n = "sec.result.unwrap_ok"(%r) : (!f) -> !sec.int
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return %n : !sec.int
  }
}
