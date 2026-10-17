// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

// Malformed schema-11 emitter facts outside operation properties are
// rejected (implementation choice MD-084, P15-58).
// expected-error@+1 {{sec.reference must be a dictionary attribute}}
func.func @reference_not_dictionary(%r: !sec.ref<!sec.int> {sec.reference = "proven"}) {
  return
}

// -----

// expected-error@+1 {{sec.reference requires borrow_id}}
func.func @reference_missing_borrow(%r: !sec.ref<!sec.int> {sec.reference = {lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "x", address_stability = "Stable", provenance = "local"}}) {
  return
}

// -----

// expected-error@+1 {{sec.reference lifetime_id must be a non-zero i32}}
func.func @reference_zero_lifetime(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 0 : i32, validity_policy = "proven", validity_proof = "x", address_stability = "Stable", provenance = "local"}}) {
  return
}

// -----

// expected-error@+1 {{a dynamic-epoch sec.reference requires invalidation_domain and epoch_dependency}}
func.func @reference_dynamic_without_epoch(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", address_stability = "Stable", provenance = "arena"}}) {
  return
}

// -----

// expected-error@+1 {{sec.reference validity_policy must be proven or dynamic-epoch}}
func.func @reference_policy(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "trusted", address_stability = "Stable", provenance = "local"}}) {
  return
}

// -----

// expected-error@+1 {{sec.reference requires non-empty address_stability and provenance}}
func.func @reference_empty_provenance(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "x", address_stability = "Stable", provenance = ""}}) {
  return
}

// -----

// expected-error@+1 {{sec.reference address_space is omitted for ordinary memory and otherwise mmio}}
func.func @reference_ordinary_space(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", address_space = "ordinary"}}) {
  return
}

// -----

// expected-error@+1 {{sec.reference origin_storages must be an array of i32}}
func.func @reference_origins(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", origin_storages = ["one"]}}) {
  return
}

// -----

// expected-error@+1 {{sec.reference pinned must be a boolean}}
func.func @reference_pinned(%r: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", pinned = 1 : i32}}) {
  return
}

// -----

func.func @reference_on_value_result(%v: !sec.int) {
  // expected-error@+1 {{sec.reference annotates an operation with one !sec.ref or !sec.ref_mut result}}
  %c = "sec.const.int"() <{value = "1"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}} : () -> !sec.int
  return
}

// -----

// expected-error@+1 {{sec.return_origins must be a non-empty array of origin strings on func.func}}
func.func @return_origins_empty(%r: !sec.ref<!sec.int>) -> !sec.ref<!sec.int> attributes {sec.return_origins = []} {
  return %r : !sec.ref<!sec.int>
}

// -----

func.func @address_taken_value() {
  // expected-error@+1 {{sec.address_taken must be a boolean attribute}}
  %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "x", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = 1 : i32} : () -> !sec.storage<!sec.int>
  return
}

// -----

func.func @address_taken_placement() {
  // expected-error@+1 {{sec.address_taken annotates sec.storage.declare}}
  %c = "sec.const.int"() <{value = "1"}> {sec.address_taken = true} : () -> !sec.int
  return
}

// -----

func.func @origin_storages_placement() {
  // expected-error@+1 {{sec.origin_storages annotates an operation defining one !sec.ref or !sec.ref_mut}}
  %c = "sec.const.int"() <{value = "1"}> {sec.origin_storages = [1 : i32]} : () -> !sec.int
  return
}

// -----

func.func @origin_storages_type(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{sec.origin_storages must be an array of i32}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> {sec.origin_storages = [1 : i64]} : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @reference_space_ordinary(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{sec.reference_address_space is omitted for ordinary memory and otherwise mmio}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> {sec.reference_address_space = "ordinary"} : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @pinned_type(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{sec.pinned must be a boolean attribute}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> {sec.pinned = "yes"} : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func private @callee(!sec.ref_mut<!sec.int>) attributes {sec.extern = false}
func.func @mutable_action(%r: !sec.ref_mut<!sec.int>) {
  // expected-error@+1 {{argument action must be mutable-reference}}
  "sec.call.direct"(%r) <{callee = @callee}> {sec.argument_actions = ["shared-reference"]} : (!sec.ref_mut<!sec.int>) -> ()
  return
}

// -----

func.func private @callee(!sec.int) attributes {sec.extern = false}
func.func @value_reference_action(%v: !sec.int) {
  // expected-error@+1 {{argument action must be copy-trivial}}
  "sec.call.direct"(%v) <{callee = @callee}> {sec.argument_actions = ["shared-reference"]} : (!sec.int) -> ()
  return
}
