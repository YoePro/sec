// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-084 sections 2(4)-(5),3(5): one consistent source of reference facts.
module {
  func.func @conflict(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'borrow_id' conflicts with the operation property}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.reference = {borrow_id = 3 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @conflict(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'lifetime_id' conflicts with the operation property}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 3 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @conflict(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'invalidation_domain' conflicts with the operation property}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @conflict(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'epoch_dependency' conflicts with the operation property}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 3 : i32, address_stability = "Stable", provenance = "local"}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @conflict(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'address_stability' conflicts with the operation property}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Fixed", provenance = "local"}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @conflict(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'provenance' conflicts with the operation property}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "different"}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @conflict(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'validity_policy' conflicts with the operation property}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", address_stability = "Stable", provenance = "local", validity_proof = "lexical-borrow"}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @supplemental(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'pinned' conflicts with sec.pinned}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.pinned = false, sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local", pinned = true}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @supplemental(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{sec.reference field 'origin_storages' conflicts with sec.origin_storages}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.origin_storages = [2 : i32], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local", origin_storages = [1 : i32]}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @invalid(%p: !sec.place<si32, "ro">) {
    // expected-error@+1 {{address_stability must be Movable, Stable, Fixed or Unknown}}
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Pinned", provenance = "local"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  // expected-error@+1 {{address_stability must be Movable, Stable, Fixed or Unknown}}
  func.func private @invalid(!sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Pinned", provenance = "local"}})
}

// -----

module {
  func.func @false_marker() {
    // expected-error@+1 {{sec.address_taken must be the boolean marker true}}
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "x", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = false} : () -> !sec.storage<si32>
    return
  }
}
