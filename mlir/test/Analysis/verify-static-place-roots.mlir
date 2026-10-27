// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | FileCheck %s
// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s --check-prefix=ROUND

// MD-116 (dialect v11.41.3): a static Place root names its address-taken
// sec.static.global by symbol, and a returned reference borrowed from it is
// summarized as static@N with N the explicit sec.static_id, not the Place
// root, the storage identity or the symbol suffix. A direct call in the same
// module forwards the callee's static origin.

// ROUND: "sec.static.global"() <{initial = "10", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.address_taken = true, {{.*}}sec.static_id = 4 : i32}
// ROUND: "sec.place.static"() <{address_space = "ordinary", global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32}>
"sec.static.global"() <{initial = "10", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true, sec.source_name = "main::Count"} : () -> ()
"sec.static.global"() <{initial = "0", is_mutable = true, sym_name = "sec_static_2", type = si32}> {sec.static_id = 5 : i32, sec.address_taken = true, sec.source_name = "main::Total"} : () -> ()
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_3", type = si32}> {sec.static_id = 6 : i32, sec.source_name = "main::Plain"} : () -> ()

// CHECK-LABEL: func.func @count
func.func @count() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4"]} {
  %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %r = "sec.ref.borrow_shared"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  return %r : !sec.ref<si32>
}

// CHECK-LABEL: func.func @total
func.func @total() -> !sec.ref_mut<si32> attributes {sec.extern = false, sec.return_origins = ["static@5"]} {
  %root = "sec.place.static"() <{global = @sec_static_2, place_root_id = 13 : i32, storage_identity = 8 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "rw">
  %r = "sec.ref.borrow_mut"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "rw">) -> !sec.ref_mut<si32>
  return %r : !sec.ref_mut<si32>
}

// A shared view of a mutable static and a choice between two statics.
// CHECK-LABEL: func.func @either
func.func @either(%flag: i1) -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4", "static@5"]} {
  cf.cond_br %flag, ^a, ^b
^a:
  %one = "sec.place.static"() <{global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %r1 = "sec.ref.borrow_shared"(%one) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  cf.br ^join(%r1 : !sec.ref<si32>)
^b:
  %two = "sec.place.static"() <{global = @sec_static_2, place_root_id = 13 : i32, storage_identity = 8 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %r2 = "sec.ref.borrow_shared"(%two) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  cf.br ^join(%r2 : !sec.ref<si32>)
^join(%r: !sec.ref<si32>):
  return %r : !sec.ref<si32>
}

// CHECK-LABEL: func.func @forward
func.func @forward() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4"]} {
  %r = "sec.call.direct"() <{callee = @count}> {sec.argument_actions = [], sec.reference = {borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}} : () -> !sec.ref<si32>
  return %r : !sec.ref<si32>
}

// Reading a static Place does not need the static to be returned.
// CHECK-LABEL: func.func @read
func.func @read() -> si32 {
  %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %value = "sec.place.read"(%root) <{action = "copy-trivial"}> : (!sec.place<si32, "ro">) -> si32
  return %value : si32
}

// MD-116 §2(2)/§4(6): a new Place root may denote the same static storage.
func.func @same_storage_new_root() -> !sec.ref<si32> attributes {sec.extern = false, sec.return_origins = ["static@4"]} {
  %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 999 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  %r = "sec.ref.borrow_shared"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
  return %r : !sec.ref<si32>
}
