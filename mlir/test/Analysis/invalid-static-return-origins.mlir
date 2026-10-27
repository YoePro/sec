// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics --sec-verify-borrow-semantics -o /dev/null

// MD-116 §§ 6, 9(3): a static return origin is the explicit sec.static_id of
// a traced static Place root. A summary naming another static, a local
// masquerading as a static, an inadmissible projection and a static origin
// from a callee declaration that cannot be linked are rejected.

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true} : () -> ()
"sec.static.global"() <{initial = "2", is_mutable = false, sym_name = "sec_static_2", type = si32}> {sec.static_id = 5 : i32, sec.address_taken = true} : () -> ()
func.func @wrong_static() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@5"]} {
  %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %r = "sec.ref.borrow_shared"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  // expected-error@+1 {{returns a reference derived from static@4, which sec.return_origins does not list}}
  return %r : !sec.ref<si32>
}

// -----

// The symbol suffix is never an identity: static@1 does not name @sec_static_1.
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true} : () -> ()
// expected-error@+1 {{entry "static@1" must name an address-taken static with that sec.static_id}}
func.func @symbol_suffix() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@1"]} {
  %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %r = "sec.ref.borrow_shared"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  return %r : !sec.ref<si32>
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true} : () -> ()
func.func @local_as_static(%storage: !sec.storage<si32>) -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4"]} {
  %root = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<si32>) -> !sec.place<si32, "ro">
  %r = "sec.ref.borrow_shared"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  // expected-error@+1 {{returns a reference to local storage or a local value}}
  return %r : !sec.ref<si32>
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true} : () -> ()
// expected-error@+1 {{entry "static@4.field#0" has a projection that the static's type does not admit}}
func.func @bad_projection() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4.field#0"]} {
  %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %r = "sec.ref.borrow_shared"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  return %r : !sec.ref<si32>
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true} : () -> ()
func.func private @imported() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4"]}
func.func @through_declaration() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4"]} {
  %r = "sec.call.direct"() <{callee = @imported}> {sec.argument_actions = [], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}} : () -> !sec.ref<si32>
  // expected-error@+1 {{cannot instantiate origin "static@4" of callee @imported at this call}}
  return %r : !sec.ref<si32>
}
