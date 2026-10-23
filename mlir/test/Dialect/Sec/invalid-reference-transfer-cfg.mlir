// RUN: sec-mlir-opt %s --sec-verify-reference-guards -split-input-file -verify-diagnostics -o /dev/null
// RUN: sec-mlir-opt %s --sec-verify-borrow-semantics -split-input-file -verify-diagnostics -o /dev/null
// P15 section 7: joins and cycles cannot hide dynamic reference dependencies.
module {
  func.func @copy_shared_strengthened(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.copy_shared"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @copy_shared_different_domain(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.copy_shared"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @copy_shared_different_epoch(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 4 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.copy_shared"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @copy_shared_mixed(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.copy_shared"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @move_strengthened(%flag: i1, %left: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref_mut<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref_mut<si32>)
  ^join(%merged: !sec.ref_mut<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.move"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @move_different_domain(%flag: i1, %left: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref_mut<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref_mut<si32>)
  ^join(%merged: !sec.ref_mut<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.move"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @move_different_epoch(%flag: i1, %left: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 4 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref_mut<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref_mut<si32>)
  ^join(%merged: !sec.ref_mut<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.move"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @move_mixed(%flag: i1, %left: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref_mut<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref_mut<si32>)
  ^join(%merged: !sec.ref_mut<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.move"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @loop_strengthened(%flag: i1, %source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^loop(%source : !sec.ref<si32>)
  ^loop(%carried: !sec.ref<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.copy_shared"(%carried) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "proven", validity_proof = "lexical-borrow"}> : (!sec.ref<si32>) -> !sec.ref<si32>
    cf.cond_br %flag, ^loop(%carried : !sec.ref<si32>), ^exit
  ^exit:
    return
  }
}

// -----

module {
  func.func @loop_replaced_epoch(%flag: i1, %source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^loop(%source : !sec.ref<si32>)
  ^loop(%carried: !sec.ref<si32>):
    // expected-error@+1 {{reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency}}
    %copy = "sec.ref.copy_shared"(%carried) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 9 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    cf.cond_br %flag, ^loop(%carried : !sec.ref<si32>), ^exit
  ^exit:
    return
  }
}
