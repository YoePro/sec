// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | FileCheck %s

// Well-formed schema-11 Place/reference CFGs pass the three P15 verifiers
// (sec_mlir_dialect_v11.md sections 35-37).

// CHECK-LABEL: func.func @guarded_element
func.func @guarded_element(%storage: !sec.storage<!sec.array<!sec.int, "3">>, %index: !sec.int) -> !sec.int {
  %array = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.array<!sec.int, "3">>) -> !sec.place<!sec.array<!sec.int, "3">, "rw">
  %in = "sec.place.array_index_in_bounds"(%array, %index) <{index_signed = true}> : (!sec.place<!sec.array<!sec.int, "3">, "rw">, !sec.int) -> i1
  cf.cond_br %in, ^ok, ^bad
^ok:
  %element = "sec.place.array_element"(%array, %index) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.int, "3">, "rw">, !sec.int) -> !sec.place<!sec.int, "rw">
  %value = "sec.place.read"(%element) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
  return %value : !sec.int
^bad:
  "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
}

// CHECK-LABEL: func.func @guarded_payload
func.func @guarded_payload(%option: !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> !sec.int {
  %place = "sec.place.value"(%option) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> !sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">
  %some = "sec.union.is_variant"(%option) <{variant = 0 : i32}> : (!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> i1
  cf.cond_br %some, ^some, ^none
^some:
  %payload = "sec.place.union_payload"(%place) <{variant = 0 : i32}> : (!sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  %reference = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  %through = "sec.place.deref"(%reference) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
  %value = "sec.place.read"(%through) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
  "sec.ref.end_borrow"(%reference) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
  return %value : !sec.int
^none:
  %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.int
  return %zero : !sec.int
}

// CHECK-LABEL: func.func @result_ok_payload
func.func @result_ok_payload(%result: !sec.result<!sec.int, !sec.string>) -> !sec.int {
  %place = "sec.place.value"(%result) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.result<!sec.int, !sec.string>) -> !sec.place<!sec.result<!sec.int, !sec.string>, "ro">
  %err = "sec.result.is_err"(%result) : (!sec.result<!sec.int, !sec.string>) -> i1
  cf.cond_br %err, ^err, ^ok
^ok:
  %payload = "sec.place.union_payload"(%place) <{variant = 0 : i32}> : (!sec.place<!sec.result<!sec.int, !sec.string>, "ro">) -> !sec.place<!sec.int, "ro">
  %value = "sec.place.read"(%payload) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
  return %value : !sec.int
^err:
  %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.int
  return %zero : !sec.int
}

// CHECK-LABEL: func.func @dynamic_guarded
func.func @dynamic_guarded(%storage: !sec.storage<!sec.int>) -> !sec.int {
  %place = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %reference = "sec.ref.borrow_mut"(%place) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
  %valid = "sec.ref.is_valid"(%reference) : (!sec.ref_mut<!sec.int>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  %through = "sec.place.deref"(%reference) : (!sec.ref_mut<!sec.int>) -> !sec.place<!sec.int, "rw">
  %value = "sec.place.read"(%through) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
  %shared = "sec.ref.reborrow_shared"(%reference) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena", source_borrow = 1 : i32}> : (!sec.ref_mut<!sec.int>) -> !sec.ref<!sec.int>
  "sec.ref.end_borrow"(%shared) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<!sec.int>) -> ()
  return %value : !sec.int
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}

// A dynamic-epoch reference parameter names its facts in sec.reference and
// is guarded like any other dynamic reference.
// CHECK-LABEL: func.func @dynamic_parameter
func.func @dynamic_parameter(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}}) -> !sec.int {
  %valid = "sec.ref.is_valid"(%r) : (!sec.ref<!sec.int>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  %place = "sec.place.deref"(%r) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
  return %value : !sec.int
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}

// Returned references derive only from the parameters sec.return_origins
// names, also through block arguments and calls.
// CHECK-LABEL: func.func @pick
func.func @pick(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %b: !sec.ref<!sec.int> {sec.reference = {borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %first: i1) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0", "param#1"]} {
  cf.cond_br %first, ^join(%a : !sec.ref<!sec.int>), ^join(%b : !sec.ref<!sec.int>)
^join(%r: !sec.ref<!sec.int>):
  return %r : !sec.ref<!sec.int>
}

// CHECK-LABEL: func.func @forward
func.func @forward(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %first: i1) -> !sec.ref<!sec.int> attributes {sec.extern = false, sec.return_origins = ["param#0"]} {
  // CHECK: sec.argument_actions = ["shared-reference", "shared-reference", "copy-trivial"]
  %r = "sec.call.direct"(%a, %a, %first) <{callee = @pick}> {sec.argument_actions = ["shared-reference", "shared-reference", "copy-trivial"], sec.reference = {borrow_id = 4 : i32, lifetime_id = 4 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}} : (!sec.ref<!sec.int>, !sec.ref<!sec.int>, i1) -> !sec.ref<!sec.int>
  return %r : !sec.ref<!sec.int>
}
