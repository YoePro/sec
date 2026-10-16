// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

// Local contracts of the schema-11 Place and reference operations
// (sec_mlir_dialect_v11.md sections 8-26).

func.func @value_rw(%v: !sec.int) {
  // expected-error@+1 {{a value Place root is read-only}}
  %p = "sec.place.value"(%v) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.int) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @root_id(%v: !sec.int) {
  // expected-error@+1 {{place_root_id must be non-zero}}
  %p = "sec.place.value"(%v) <{place_root_id = 0 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.int) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @field_widens(%pair: !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) {
  // expected-error@+1 {{a field Place cannot exceed its base authority}}
  %f = "sec.place.field"(%pair) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @payload_type(%option: !sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) {
  // expected-error@+1 {{result Place must denote the variant payload type}}
  %p = "sec.place.union_payload"(%option) <{variant = 0 : i32}> : (!sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) -> !sec.place<!sec.string, "ro">
  return
}

// -----

func.func @payload_empty(%option: !sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) {
  // expected-error@+1 {{the selected variant has no payload}}
  %p = "sec.place.union_payload"(%option) <{variant = 1 : i32}> : (!sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @write_ro(%place: !sec.place<!sec.int, "ro">, %v: !sec.int) {
  // expected-error@+1 {{a write requires a rw Place}}
  "sec.place.write"(%place, %v) : (!sec.place<!sec.int, "ro">, !sec.int) -> ()
  return
}

// -----

func.func @read_action(%place: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{schema-11 Place reads are copy-trivial}}
  %v = "sec.place.read"(%place) <{action = "move"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
  return
}

// -----

func.func @deref_authority(%r: !sec.ref<!sec.int>) {
  // expected-error@+1 {{!sec.ref dereferences to a ro Place and !sec.ref_mut to a rw Place}}
  %p = "sec.place.deref"(%r) : (!sec.ref<!sec.int>) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @borrow_mut_ro(%place: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{a mutable borrow requires a rw Place}}
  %r = "sec.ref.borrow_mut"(%place) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref_mut<!sec.int>
  return
}

// -----

func.func @proven_without_proof(%place: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{a proven reference requires validity_proof}}
  %r = "sec.ref.borrow_shared"(%place) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @dynamic_without_epoch(%place: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{a dynamic-epoch reference requires invalidation_domain and epoch_dependency}}
  %r = "sec.ref.borrow_shared"(%place) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", address_stability = "Stable", provenance = "arena"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @copy_mut(%r: !sec.ref_mut<!sec.int>) {
  // expected-error@+1 {{copies a !sec.ref to the same !sec.ref type; !sec.ref_mut is never copied}}
  %c = "sec.ref.copy_shared"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<!sec.int>) -> !sec.ref_mut<!sec.int>
  return
}

// -----

func.func @shared_to_mut(%r: !sec.ref<!sec.int>) {
  // expected-error@+1 {{no shared-to-mutable reborrow}}
  %m = "sec.ref.reborrow_mut"(%r) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref<!sec.int>) -> !sec.ref_mut<!sec.int>
  return
}

// -----

func.func @reborrow_same_borrow(%r: !sec.ref<!sec.int>) {
  // expected-error@+1 {{a reborrow derives a new borrow_id from a distinct non-zero source_borrow}}
  %s = "sec.ref.reborrow_shared"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref<!sec.int>) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @compare_predicate(%a: !sec.ref<!sec.int>, %b: !sec.ref<!sec.int>) {
  // expected-error@+1 {{predicate must be eq or ne}}
  %c = "sec.ref.compare"(%a, %b) <{predicate = "lt"}> : (!sec.ref<!sec.int>, !sec.ref<!sec.int>) -> i1
  return
}

// -----

func.func @fail_reason() {
  // expected-error@+1 {{reason must be panic.invalid-reference-generation}}
  "sec.fail.reference_generation"() <{reason = "panic.other"}> : () -> ()
}
