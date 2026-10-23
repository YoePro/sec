// RUN: sec-mlir-opt %s --sec-verify-borrow-semantics | sec-mlir-opt | FileCheck %s
module {
  // CHECK-LABEL: func.func @copy_shared
  func.func @copy_shared(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^next(%source : !sec.ref<si32>)
  ^next(%forwarded: !sec.ref<si32>):
    // CHECK: sec.ref.copy_shared
    %target = "sec.ref.copy_shared"(%forwarded) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
  // CHECK-LABEL: func.func @move
  func.func @move(%source: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^next(%source : !sec.ref_mut<si32>)
  ^next(%forwarded: !sec.ref_mut<si32>):
    // CHECK: sec.ref.move
    %target = "sec.ref.move"(%forwarded) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}
