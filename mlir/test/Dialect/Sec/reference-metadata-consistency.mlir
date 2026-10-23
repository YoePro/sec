// RUN: sec-mlir-opt %s -split-input-file | sec-mlir-opt -split-input-file -o /dev/null
// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-084 sections 2(4)-(5),3(5): one consistent source of reference facts.
module {
  func.func @matching_Movable(%p: !sec.place<si32, "ro">) {
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Movable", provenance = "local"}> {sec.pinned = true, sec.origin_storages = [1 : i32], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Movable", provenance = "local", pinned = true, origin_storages = [1 : i32]}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @matching_Stable(%p: !sec.place<si32, "ro">) {
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local"}> {sec.pinned = true, sec.origin_storages = [1 : i32], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "local", pinned = true, origin_storages = [1 : i32]}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @matching_Fixed(%p: !sec.place<si32, "ro">) {
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Fixed", provenance = "local"}> {sec.pinned = true, sec.origin_storages = [1 : i32], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Fixed", provenance = "local", pinned = true, origin_storages = [1 : i32]}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @matching_Unknown(%p: !sec.place<si32, "ro">) {
    %r = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Unknown", provenance = "local"}> {sec.pinned = true, sec.origin_storages = [1 : i32], sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Unknown", provenance = "local", pinned = true, origin_storages = [1 : i32]}} : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return
  }
}
