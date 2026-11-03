// RUN: sec-mlir-opt %s --canonicalize --cse --sec-verify-ownership --sec-verify-cleanups --sec-verify-destruction-plans | FileCheck %s
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — section 136 (P17-57).
// Generic simplification keeps every ownership operation that is not a pure
// trivial copy: a move, a payload move and a destruction survive it in order.

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  // CHECK-LABEL: func.func @kept
  // CHECK: sec.own.move
  // CHECK: sec.union.unwrap_payload
  // CHECK: sec.own.destroy_value
  func.func @kept() {
    %h = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    %m = "sec.own.move"(%h) : (!h) -> !h
    "sec.call.direct"(%m) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !o
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^none
    ^some:
    %p = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%p) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^none:
    "sec.own.destroy_value"(%o) <{destruction_plan = "o", cause = "temporary-end"}> : (!o) -> ()
    return
  }
}
