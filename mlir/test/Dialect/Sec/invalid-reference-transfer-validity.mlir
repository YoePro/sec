// RUN: sec-mlir-opt %s --sec-verify-reference-guards -split-input-file -verify-diagnostics -o /dev/null
// RUN: sec-mlir-opt %s --sec-verify-borrow-semantics -split-input-file -verify-diagnostics -o /dev/null

// -----

module {
  func.func @invalid(%p: !sec.place<si32, "rw">) {
    %source = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.place<si32, "rw">) -> !sec.ref<si32>
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @invalid(%p: !sec.place<si32, "rw">) {
    %source = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.place<si32, "rw">) -> !sec.ref<si32>
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 2 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @invalid(%p: !sec.place<si32, "rw">) {
    %source = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.place<si32, "rw">) -> !sec.ref<si32>
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 4 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @invalid(%p: !sec.place<si32, "rw">) {
    %source = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.place<si32, "rw">) -> !sec.ref_mut<si32>
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.move"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @invalid(%p: !sec.place<si32, "rw">) {
    %source = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.place<si32, "rw">) -> !sec.ref_mut<si32>
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.move"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 2 : i32}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @invalid(%p: !sec.place<si32, "rw">) {
    %source = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.place<si32, "rw">) -> !sec.ref_mut<si32>
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.move"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 4 : i32}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @parameter(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @parameter(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^next(%source : !sec.ref<si32>)
  ^next(%forwarded: !sec.ref<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %target = "sec.ref.copy_shared"(%forwarded) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}
