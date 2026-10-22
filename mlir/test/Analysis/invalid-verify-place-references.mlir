// RUN: sec-mlir-opt %s -split-input-file --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics -verify-diagnostics -o /dev/null

// Each P15 verifier rule rejects its violation (sec_mlir_dialect_v11.md
// sections 29, 35-37).

// expected-error@+1 {{a !sec.place cannot be returned}}
func.func @return_place(%v: !sec.int) -> !sec.place<!sec.int, "ro"> {
  %p = "sec.place.value"(%v) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.int) -> !sec.place<!sec.int, "ro">
  return %p : !sec.place<!sec.int, "ro">
}

// -----

func.func @escape_through_branch(%v: !sec.int) {
  %p = "sec.place.value"(%v) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.int) -> !sec.place<!sec.int, "ro">
  // expected-error@+1 {{a !sec.place may only be consumed by Place and borrow operations}}
  cf.br ^next(%p : !sec.place<!sec.int, "ro">)
^next(%q: !sec.place<!sec.int, "ro">):
  return
}

// -----

func.func @unguarded_element(%s: !sec.storage<!sec.array<!sec.int, "3">>, %i: !sec.int) {
  %a = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.array<!sec.int, "3">>) -> !sec.place<!sec.array<!sec.int, "3">, "rw">
  // expected-error@+1 {{runtime-check element Place requires a dominating true edge}}
  %e = "sec.place.array_element"(%a, %i) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.int, "3">, "rw">, !sec.int) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @wrong_variant(%o: !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) {
  %p = "sec.place.value"(%o) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> !sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">
  %none = "sec.union.is_variant"(%o) <{variant = 1 : i32}> : (!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> i1
  cf.cond_br %none, ^none, ^other
^none:
  // expected-error@+1 {{payload Place requires a dominating active-variant test of the same Place}}
  %pl = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  return
^other:
  return
}

// -----

func.func @ok_on_err_edge(%r: !sec.result<!sec.int, !sec.string>) {
  %p = "sec.place.value"(%r) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.result<!sec.int, !sec.string>) -> !sec.place<!sec.result<!sec.int, !sec.string>, "ro">
  %err = "sec.result.is_err"(%r) : (!sec.result<!sec.int, !sec.string>) -> i1
  cf.cond_br %err, ^err, ^ok
^err:
  // expected-error@+1 {{payload Place requires a dominating active-variant test of the same Place}}
  %pl = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!sec.result<!sec.int, !sec.string>, "ro">) -> !sec.place<!sec.int, "ro">
  return
^ok:
  return
}

// -----

func.func @dynamic_unguarded(%s: !sec.storage<!sec.int>) {
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
  // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
  %t = "sec.place.deref"(%r) : (!sec.ref_mut<!sec.int>) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @false_edge_not_failure(%s: !sec.storage<!sec.int>) {
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
  %v = "sec.ref.is_valid"(%r) : (!sec.ref_mut<!sec.int>) -> i1
  cf.cond_br %v, ^ok, ^stale
^ok:
  // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
  %t = "sec.place.deref"(%r) : (!sec.ref_mut<!sec.int>) -> !sec.place<!sec.int, "rw">
  return
^stale:
  return
}

// -----

func.func @wrong_source_borrow(%s: !sec.storage<!sec.int>) {
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
  // expected-error@+1 {{source_borrow must name the borrow_id of the reference it derives from}}
  %c = "sec.ref.reborrow_shared"(%r) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 2 : i32}> : (!sec.ref_mut<!sec.int>) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @use_after_end(%s: !sec.storage<!sec.int>) {
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
  "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
  // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
  %t = "sec.place.deref"(%r) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @wrong_end_ids(%s: !sec.storage<!sec.int>) {
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
  // expected-error@+1 {{end_borrow must name the ended reference's borrow_id and lifetime_id}}
  "sec.ref.end_borrow"(%r) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<!sec.int>) -> ()
  return
}

// -----

func.func @use_after_move(%s: !sec.storage<!sec.int>) {
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
  %m = "sec.ref.move"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<!sec.int>) -> !sec.ref_mut<!sec.int>
  // expected-error@+1 {{!sec.ref_mut used after it was moved}}
  %t = "sec.place.deref"(%r) : (!sec.ref_mut<!sec.int>) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @dynamic_reborrow_unguarded(%s: !sec.storage<!sec.int>) {
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
  // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
  %c = "sec.ref.reborrow_shared"(%r) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena", source_borrow = 1 : i32}> : (!sec.ref_mut<!sec.int>) -> !sec.ref<!sec.int>
  return
}


// -----

func.func @dynamic_parameter_unguarded(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "arena"}}) -> !sec.int {
  // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
  %place = "sec.place.deref"(%r) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
  return %value : !sec.int
}

// -----

// expected-error@+1 {{a function returning a direct reference requires sec.return_origins}}
func.func @missing_return_origins(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> {
  return %a : !sec.ref<!sec.int>
}

// -----

func.func @unnamed_parameter(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %b: !sec.ref<!sec.int> {sec.reference = {borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<!sec.int> attributes {sec.return_origins = ["param#0"]} {
  // expected-error@+1 {{returns a reference derived from param#1, which sec.return_origins does not list}}
  return %b : !sec.ref<!sec.int>
}

// -----

func.func @return_local(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %s: !sec.storage<!sec.int>) -> !sec.ref<!sec.int> attributes {sec.return_origins = ["param#0"]} {
  %place = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.ref.borrow_shared"(%place) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
  // expected-error@+1 {{returns a reference to local storage or a local value}}
  return %r : !sec.ref<!sec.int>
}

// -----

// expected-error@+1 {{must name a reference parameter}}
func.func @origin_not_reference(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %flag: i1) -> !sec.ref<!sec.int> attributes {sec.return_origins = ["param#1"]} {
  return %a : !sec.ref<!sec.int>
}

// -----

func.func private @callee(!sec.ref<!sec.int>) attributes {sec.extern = false}
func.func @wrong_action(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  // expected-error@+1 {{argument action must be shared-reference}}
  "sec.call.direct"(%a) <{callee = @callee}> {sec.argument_actions = ["copy-trivial"]} : (!sec.ref<!sec.int>) -> ()
  return
}

// -----

// expected-error@+1 {{sec.reference annotates a !sec.ref or !sec.ref_mut function parameter}}
func.func @reference_on_value(%v: !sec.int {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  return
}

// -----

// expected-error@+1 {{a proven sec.reference names validity_proof and no epoch_dependency}}
func.func @proven_without_proof(%a: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", address_stability = "Stable", provenance = "parameter"}}) {
  return
}

// -----

func.func @storage_not_address_taken() -> !sec.int {
  %v = "sec.const.int"() <{value = "1"}> : () -> !sec.int
  %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "x", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!sec.int>
  "sec.storage.init"(%s, %v) : (!sec.storage<!sec.int>, !sec.int) -> ()
  // expected-error@+1 {{a Place root requires its storage declaration to carry sec.address_taken = true}}
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  %r = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
  return %r : !sec.int
}
