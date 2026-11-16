// RUN: not sec-mlir-opt %s --sec-verify-ownership 2>&1 | FileCheck %s --check-prefix=LOST
// RUN: sec-mlir-opt %s --sec-annotate-successor-actions --sec-verify-ownership | FileCheck %s
// RUN: sec-mlir-opt %s --sec-annotate-successor-actions --canonicalize --sec-annotate-successor-actions --sec-verify-ownership | FileCheck %s --check-prefix=OPT
// P17-57: `sec.successor_actions` is discardable branch metadata (dialect v13
// section 26) that a branch-rewriting optimization drops. The action of a
// forwarded operand is fixed by its type, so --sec-annotate-successor-actions
// restores exactly the emitted metadata before ownership verification.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — section 26.

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
  // LOST: a forwarded branch operand needs destruction, so its ownership action must be "move"
  // CHECK-LABEL: func.func @joined
  // CHECK: cf.br ^bb3({{.*}}) {sec.successor_actions = {{\[\[}}"move", "copy"]]}
  // CHECK: cf.br ^bb3({{.*}}) {sec.successor_actions = {{\[\[}}"move", "copy"]]}
  // OPT-LABEL: func.func @joined
  func.func @joined(%c: i1, %n: i32) {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    cf.cond_br %c, ^bb1, ^bb2
    ^bb1:
    %w = "sec.own.move"(%v) : (!h) -> !h
    cf.br ^bb3(%w, %n : !h, i32)
    ^bb2:
    cf.br ^bb3(%v, %n : !h, i32)
    ^bb3(%m: !h, %k: i32):
    "sec.call.direct"(%m) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}
