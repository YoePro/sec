// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-084 sections 2(3),3(1)-(2): exact reference metadata contract.
module {
  // expected-error@+1 {{unknown sec.reference field 'relocation_class'}}
  func.func private @unknown(!sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "parameter", relocation_class = "Fixed"}})
}

// -----

module {
  // expected-error@+1 {{unknown sec.reference field 'validty_policy'}}
  func.func private @unknown(!sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "parameter", validty_policy = "proven"}})
}

// -----

module {
  // expected-error@+1 {{unknown sec.reference field 'arbitrary_fact'}}
  func.func private @unknown(!sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "parameter", arbitrary_fact = true}})
}

// -----

module {
  // expected-error@+1 {{sec.reference validity_proof must be a string}}
  func.func private @bad_proof(!sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "parameter", validity_proof = 1 : i32}})
}

// -----

module {
  // expected-error@+1 {{sec.return_origins requires a function with a reference result}}
  func.func private @void_result(!sec.ref<si32>) attributes {sec.return_origins = ["param#0"]}
}

// -----

module {
  // expected-error@+1 {{sec.return_origins requires a function with a reference result}}
  func.func private @integer_result(!sec.ref<si32>) -> si32 attributes {sec.return_origins = ["param#0"]}
}

// -----

module {
  // expected-error@+1 {{sec.return_origins requires a function with a reference result}}
  func.func private @owned_register_result(!sec.ref<si32>) -> !sec.register<"device::Control"> attributes {sec.return_origins = ["param#0"]}
}
