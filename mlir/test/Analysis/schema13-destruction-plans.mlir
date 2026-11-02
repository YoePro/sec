// RUN: sec-mlir-opt %s --sec-verify-destruction-plans | FileCheck %s
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 116, 123.

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">

// CHECK-LABEL: module
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [
  {id = "scalar", type = !sec.int, kind = "trivial"},
  {id = "ref", type = !sec.ref<!h>, kind = "trivial"},
  {id = "slice", type = !sec.slice<!h>, kind = "trivial"},
  {id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free},
  {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]},
  {id = "a", type = !a, kind = "fixed-array", element = "h", element_count = "1000000"},
  {id = "z", type = !z, kind = "fixed-array", element = "h", element_count = "0"},
  {id = "nested", type = !sec.array<!p, "2">, kind = "fixed-array", element = "p", element_count = "2"},
  {id = "o", type = !o, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]},
  {id = "r", type = !r, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}
]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  // CHECK-LABEL: func.func @uses
  func.func @uses(%p: !sec.place<!p, "rw">, %v: !h, %q: !sec.int) {
    "sec.own.destroy_place"(%p) <{destruction_plan = "p", cause = "function-exit", skipped = ["field#2"]}> : (!sec.place<!p, "rw">) -> ()
    "sec.own.destroy_value"(%v) <{destruction_plan = "h", cause = "function-exit"}> : (!h) -> ()
    "sec.own.discard"(%q) <{discard_origin = "explicit"}> : (!sec.int) -> ()
    return
  }
}
