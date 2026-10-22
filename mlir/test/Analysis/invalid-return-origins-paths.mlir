// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics --sec-verify-borrow-semantics -o /dev/null

// MD-085 § 9(3)–(4): wrong field ordinal with matching types, wrong constant
// element, a missing dynamic projection, a bare root that does not cover a
// projected path, a callee summary that excludes the actual operand, a call
// without a callee summary (the former all-arguments fallback), a static
// root, a receiver on a function without one, and an inadmissible projection.

// -----

func.func @wrong_field(%p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0.field#0"]} {
  %whole = "sec.place.deref"(%p) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 1 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  // expected-error@+1 {{returns a reference derived from param#0.field#1, which sec.return_origins does not list}}
  return %r : !sec.ref<!sec.int>
}
// -----

func.func @wrong_constant(%a: !sec.ref<!sec.array<!sec.int, "4">> {sec.reference = {borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %i: !sec.int) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0[1]"]} {
  %whole = "sec.place.deref"(%a) : (!sec.ref<!sec.array<!sec.int, "4">>) -> !sec.place<!sec.array<!sec.int, "4">, "ro">
  %k = "sec.const.int"() <{value = "2"}> : () -> !sec.int
  %e = "sec.place.array_element"(%whole, %k) <{bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.place<!sec.array<!sec.int, "4">, "ro">, !sec.int) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%e) <{borrow_id = 4 : i32, lifetime_id = 4 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  // expected-error@+1 {{returns a reference derived from param#0[2], which sec.return_origins does not list}}
  return %r : !sec.ref<!sec.int>
}
// -----

func.func @missing_dynamic(%a: !sec.ref<!sec.array<!sec.int, "4">> {sec.reference = {borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %i: !sec.int) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0"]} {
  %whole = "sec.place.deref"(%a) : (!sec.ref<!sec.array<!sec.int, "4">>) -> !sec.place<!sec.array<!sec.int, "4">, "ro">
  %k = "sec.const.int"() <{value = "2"}> : () -> !sec.int
  %e = "sec.place.array_element"(%whole, %k) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.int, "4">, "ro">, !sec.int) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%e) <{borrow_id = 4 : i32, lifetime_id = 4 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  // expected-error@+1 {{returns a reference derived from param#0[*], which sec.return_origins does not list}}
  return %r : !sec.ref<!sec.int>
}
// -----

func.func @bare_root(%p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0"]} {
  %whole = "sec.place.deref"(%p) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  // expected-error@+1 {{returns a reference derived from param#0.field#0, which sec.return_origins does not list}}
  return %r : !sec.ref<!sec.int>
}
// -----

func.func @first(%x: !sec.ref<!sec.int> {sec.reference = {borrow_id = 11 : i32, lifetime_id = 11 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %y: !sec.ref<!sec.int> {sec.reference = {borrow_id = 12 : i32, lifetime_id = 12 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0"]} {
  return %x : !sec.ref<!sec.int>
}
func.func @callee_excludes(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 13 : i32, lifetime_id = 13 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %b: !sec.ref<!sec.int> {sec.reference = {borrow_id = 14 : i32, lifetime_id = 14 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#1"]} {
  %r = "sec.call.direct"(%a, %b) <{callee = @first}> {sec.argument_actions = ["shared-reference", "shared-reference"], sec.reference = {borrow_id = 15 : i32, lifetime_id = 15 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.ref<!sec.int>, !sec.ref<!sec.int>) -> !sec.ref<!sec.int>
  // expected-error@+1 {{returns a reference derived from param#0, which sec.return_origins does not list}}
  return %r : !sec.ref<!sec.int>
}
// -----

func.func private @opaque(!sec.ref<!sec.int>, !sec.ref<!sec.int>) -> !sec.ref<!sec.int> attributes {sec.extern = false}
func.func @no_callee_summary(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 13 : i32, lifetime_id = 13 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %b: !sec.ref<!sec.int> {sec.reference = {borrow_id = 14 : i32, lifetime_id = 14 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0", "param#1"]} {
  %r = "sec.call.direct"(%a, %b) <{callee = @opaque}> {sec.argument_actions = ["shared-reference", "shared-reference"], sec.reference = {borrow_id = 15 : i32, lifetime_id = 15 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.ref<!sec.int>, !sec.ref<!sec.int>) -> !sec.ref<!sec.int>
  // expected-error@+1 {{returns a reference from a call to opaque without a sec.return_origins summary}}
  return %r : !sec.ref<!sec.int>
}
// -----

// expected-error@+1 {{names a static root, which has no schema-11 Place root}}
func.func @static_root(%p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["static@3"]} {
  %whole = "sec.place.deref"(%p) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}
// -----

// expected-error@+1 {{names a receiver, but the function has none}}
func.func @receiver_without(%p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["receiver.field#0"]} {
  %whole = "sec.place.deref"(%p) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}
// -----

// expected-error@+1 {{has a projection that the parameter's referent type does not admit}}
func.func @inadmissible(%p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0.field#7"]} {
  %whole = "sec.place.deref"(%p) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}
// -----

// expected-error@+1 {{has a projection that the parameter's referent type does not admit}}
func.func @index_on_struct(%p: !sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0[0]"]} {
  %whole = "sec.place.deref"(%p) : (!sec.ref<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">
  %field = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %r = "sec.ref.reborrow_shared"(%field) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}
