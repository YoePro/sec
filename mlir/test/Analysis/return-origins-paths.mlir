// RUN: sec-mlir-opt %s --sec-verify-borrow-semantics | FileCheck %s

// MD-085: path-aware structural return-origin verification. Field and
// element projections extend the path, joins union their predecessors, and a
// direct call contributes only its callee's own summary instantiated over the
// actual operands (caller prefix + callee suffix).

// CHECK-LABEL: func.func @right
func.func @right(%p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0.field#1"]} {
  %whole = "sec.place.deref"(%p) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 1 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}

// CHECK-LABEL: func.func @element
func.func @element(%a: !sec.ref<!sec.array<!sec.int, "4">> {sec.reference = {borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %i: !sec.int) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0[2]", "param#0[*]"]} {
  %whole = "sec.place.deref"(%a) : (!sec.ref<!sec.array<!sec.int, "4">>) -> !sec.place<!sec.array<!sec.int, "4">, "ro">
  %two = "sec.const.int"() <{value = "2"}> : () -> !sec.int
  %fixed = "sec.place.array_element"(%whole, %two) <{bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.place<!sec.array<!sec.int, "4">, "ro">, !sec.int) -> !sec.place<!sec.int, "ro">
  %in = "sec.place.array_index_in_bounds"(%whole, %i) <{index_signed = true}> : (!sec.place<!sec.array<!sec.int, "4">, "ro">, !sec.int) -> i1
  cf.cond_br %in, ^dynamic, ^constant
^dynamic:
  %any = "sec.place.array_element"(%whole, %i) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.int, "4">, "ro">, !sec.int) -> !sec.place<!sec.int, "ro">
  %rd = "sec.ref.reborrow_shared"(%any) <{borrow_id = 4 : i32, lifetime_id = 4 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  cf.br ^join(%rd : !sec.ref<!sec.int>)
^constant:
  %rc = "sec.ref.reborrow_shared"(%fixed) <{borrow_id = 5 : i32, lifetime_id = 5 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  cf.br ^join(%rc : !sec.ref<!sec.int>)
^join(%r: !sec.ref<!sec.int>):
  return %r : !sec.ref<!sec.int>
}

// The callee promises param#0.field#1; its actual operand is this function's
// second parameter, so the composed origin is param#1.field#1.
// CHECK-LABEL: func.func @composed
func.func @composed(%unused: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 6 : i32, lifetime_id = 6 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 7 : i32, lifetime_id = 7 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#1.field#1"]} {
  %r = "sec.call.direct"(%p) <{callee = @right}> {sec.argument_actions = ["shared-reference"], sec.reference = {borrow_id = 8 : i32, lifetime_id = 8 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}

// CHECK-LABEL: func.func @method
func.func @method(%self: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 9 : i32, lifetime_id = 9 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.parameter_names = ["self"], sec.return_origins = ["receiver.field#0"]} {
  %whole = "sec.place.deref"(%self) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 10 : i32, lifetime_id = 10 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 9 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}
