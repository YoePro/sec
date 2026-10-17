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

// Malformed root, authority, field, index, variant and reference metadata
// (P15-58).

// -----

func.func @address_space(%v: !sec.int) {
  // expected-error@+1 {{address_space must be ordinary or mmio}}
  %p = "sec.place.value"(%v) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "sram"}> : (!sec.int) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @value_type(%v: !sec.int) {
  // expected-error@+1 {{Place element type must match the rooted value type}}
  %p = "sec.place.value"(%v) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.int) -> !sec.place<!sec.string, "ro">
  return
}

// -----

func.func @storage_operand(%v: !sec.int) {
  // expected-error@+1 {{operand must have !sec.storage type}}
  %p = "sec.place.storage"(%v) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.int) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @storage_type(%s: !sec.storage<!sec.int>) {
  // expected-error@+1 {{Place element type must match the storage element type}}
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.string, "rw">
  return
}

// -----

func.func @immutable_storage() {
  %s = "sec.storage.declare"() {sec.mutable = false, sec.source_name = "x", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.int>
  // expected-error@+1 {{a rw Place root requires mutable storage}}
  %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @field_not_struct(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{base Place must denote a !sec.struct}}
  %f = "sec.place.field"(%p) <{field = 0 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @field_range(%p: !sec.place<!sec.struct<identity = "main::Box", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "v", type = !sec.int, tags = []>]>, "ro">) {
  // expected-error@+1 {{struct field ordinal is out of range}}
  %f = "sec.place.field"(%p) <{field = 1 : i32}> : (!sec.place<!sec.struct<identity = "main::Box", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "v", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @field_type(%p: !sec.place<!sec.struct<identity = "main::Box", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "v", type = !sec.int, tags = []>]>, "ro">) {
  // expected-error@+1 {{result Place must denote the selected field type}}
  %f = "sec.place.field"(%p) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Box", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "v", type = !sec.int, tags = []>]>, "ro">) -> !sec.place<!sec.string, "ro">
  return
}

// -----

func.func @bounds_not_array(%p: !sec.place<!sec.int, "ro">, %i: !sec.int) {
  // expected-error@+1 {{base Place must denote a fixed !sec.array}}
  %b = "sec.place.array_index_in_bounds"(%p, %i) <{index_signed = true}> : (!sec.place<!sec.int, "ro">, !sec.int) -> i1
  return
}

// -----

func.func @bounds_signed(%p: !sec.place<!sec.array<!sec.int, "3">, "ro">, %i: !sec.int) {
  // expected-error@+1 {{index_signed must match the index semantic type}}
  %b = "sec.place.array_index_in_bounds"(%p, %i) <{index_signed = false}> : (!sec.place<!sec.array<!sec.int, "3">, "ro">, !sec.int) -> i1
  return
}

// -----

func.func @element_index(%p: !sec.place<!sec.array<!sec.int, "3">, "ro">, %i: i1) {
  // expected-error@+1 {{index must have a builtin Sec integer semantic type}}
  %e = "sec.place.array_element"(%p, %i) <{bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.place<!sec.array<!sec.int, "3">, "ro">, i1) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @element_type(%p: !sec.place<!sec.array<!sec.int, "3">, "ro">, %i: !sec.int) {
  // expected-error@+1 {{result Place must denote the array element type}}
  %e = "sec.place.array_element"(%p, %i) <{bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.place<!sec.array<!sec.int, "3">, "ro">, !sec.int) -> !sec.place<!sec.string, "ro">
  return
}

// -----

func.func @element_authority(%p: !sec.place<!sec.array<!sec.int, "3">, "ro">, %i: !sec.int) {
  // expected-error@+1 {{an element Place keeps its base authority}}
  %e = "sec.place.array_element"(%p, %i) <{bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.place<!sec.array<!sec.int, "3">, "ro">, !sec.int) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @element_kind(%p: !sec.place<!sec.array<!sec.int, "3">, "ro">, %i: !sec.int) {
  // expected-error@+1 {{bounds_kind must be proven-safe or runtime-check}}
  %e = "sec.place.array_element"(%p, %i) <{bounds_kind = "unchecked", bounds_proof = "constant"}> : (!sec.place<!sec.array<!sec.int, "3">, "ro">, !sec.int) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @element_runtime_proof(%p: !sec.place<!sec.array<!sec.int, "3">, "ro">, %i: !sec.int) {
  // expected-error@+1 {{runtime-check bounds proof must be guarded}}
  %e = "sec.place.array_element"(%p, %i) <{bounds_kind = "runtime-check", bounds_proof = "constant"}> : (!sec.place<!sec.array<!sec.int, "3">, "ro">, !sec.int) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @element_proven_proof(%p: !sec.place<!sec.array<!sec.int, "3">, "ro">, %i: !sec.int) {
  // expected-error@+1 {{proven-safe bounds proof must be constant, range, branch, contract, or analysis}}
  %e = "sec.place.array_element"(%p, %i) <{bounds_kind = "proven-safe", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.int, "3">, "ro">, !sec.int) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @result_variant(%p: !sec.place<!sec.result<!sec.int, !sec.string>, "ro">) {
  // expected-error@+1 {{a Result payload variant is 0 (Ok) or 1 (Err)}}
  %e = "sec.place.union_payload"(%p) <{variant = 2 : i32}> : (!sec.place<!sec.result<!sec.int, !sec.string>, "ro">) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @union_variant_range(%p: !sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) {
  // expected-error@+1 {{union variant index is out of range}}
  %e = "sec.place.union_payload"(%p) <{variant = 5 : i32}> : (!sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @payload_authority(%p: !sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) {
  // expected-error@+1 {{a payload Place keeps its base authority}}
  %e = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) -> !sec.place<!sec.int, "rw">
  return
}

// -----

func.func @deref_operand(%v: !sec.int) {
  // expected-error@+1 {{operand must be !sec.ref or !sec.ref_mut}}
  %p = "sec.place.deref"(%v) : (!sec.int) -> !sec.place<!sec.int, "ro">
  return
}

// -----

func.func @deref_referent(%r: !sec.ref<!sec.int>) {
  // expected-error@+1 {{result Place must denote the referent type}}
  %p = "sec.place.deref"(%r) : (!sec.ref<!sec.int>) -> !sec.place<!sec.string, "ro">
  return
}

// -----

func.func @read_type(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{result type must match the Place element type}}
  %v = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.string
  return
}

// -----

func.func @write_type(%p: !sec.place<!sec.int, "rw">, %s: !sec.string) {
  // expected-error@+1 {{value type must match the Place element type}}
  "sec.place.write"(%p, %s) : (!sec.place<!sec.int, "rw">, !sec.string) -> ()
  return
}

// -----

func.func @borrow_result(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{result must be !sec.ref of the Place element type}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref_mut<!sec.int>
  return
}

// -----

func.func @borrow_mut_result(%p: !sec.place<!sec.int, "rw">) {
  // expected-error@+1 {{result must be !sec.ref_mut of the Place element type}}
  %r = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @borrow_zero(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{borrow_id and lifetime_id must be non-zero}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 0 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @borrow_provenance(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{address_stability and provenance must be non-empty}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = ""}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @proven_epoch(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{a proven reference has no epoch_dependency}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", epoch_dependency = 1 : i32, address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @policy(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{validity_policy must be proven or dynamic-epoch}}
  %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "assumed", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @move_shared(%r: !sec.ref<!sec.int>) {
  // expected-error@+1 {{moves a !sec.ref_mut to the same !sec.ref_mut type}}
  %m = "sec.ref.move"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref<!sec.int>) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @reborrow_referent(%r: !sec.ref<!sec.int>) {
  // expected-error@+1 {{a reborrow of a reference keeps its referent type}}
  %m = "sec.ref.reborrow_shared"(%r) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref<!sec.int>) -> !sec.ref<!sec.string>
  return
}

// -----

func.func @reborrow_mut_ro_place(%p: !sec.place<!sec.int, "ro">) {
  // expected-error@+1 {{a mutable sub-Place reborrow requires a rw Place}}
  %m = "sec.ref.reborrow_mut"(%p) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.place<!sec.int, "ro">) -> !sec.ref_mut<!sec.int>
  return
}

// -----

func.func @reborrow_source(%v: !sec.int) {
  // expected-error@+1 {{source must be a reference or a dereferenced sub-Place}}
  %m = "sec.ref.reborrow_shared"(%v) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.int) -> !sec.ref<!sec.int>
  return
}

// -----

func.func @is_valid_operand(%v: !sec.int) {
  // expected-error@+1 {{operand must be !sec.ref or !sec.ref_mut}}
  %b = "sec.ref.is_valid"(%v) : (!sec.int) -> i1
  return
}

// -----

func.func @compare_referents(%a: !sec.ref<!sec.int>, %b: !sec.ref<!sec.string>) {
  // expected-error@+1 {{compares direct references to the same referent type}}
  %e = "sec.ref.compare"(%a, %b) <{predicate = "eq"}> : (!sec.ref<!sec.int>, !sec.ref<!sec.string>) -> i1
  return
}

// -----

func.func @end_borrow_zero(%r: !sec.ref<!sec.int>) {
  // expected-error@+1 {{borrow_id and lifetime_id must be non-zero}}
  "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 0 : i32}> : (!sec.ref<!sec.int>) -> ()
  return
}

// -----

func.func @end_borrow_operand(%v: !sec.int) {
  // expected-error@+1 {{operand must be !sec.ref or !sec.ref_mut}}
  "sec.ref.end_borrow"(%v) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.int) -> ()
  return
}

// -----

func.func @failure_not_last() {
  // expected-error@+1 {{must be the last operation in the parent block}}
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  return
}
