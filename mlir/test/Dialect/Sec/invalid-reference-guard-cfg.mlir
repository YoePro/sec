// RUN: sec-mlir-opt %s --sec-verify-reference-guards -split-input-file -verify-diagnostics -o /dev/null
// Schema 11 section 36: CFG forwarding cannot erase dynamic validity.
module {
  func.func @join_deref(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %place = "sec.place.deref"(%merged) : (!sec.ref<si32>) -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
  func.func @join_compare(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %same = "sec.ref.compare"(%merged, %merged) <{predicate = "eq"}> : (!sec.ref<si32>, !sec.ref<si32>) -> i1
    return
  }
}

// -----

module {
  func.func @join_reborrow(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %child = "sec.ref.reborrow_shared"(%merged) <{borrow_id = 2 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, source_borrow = 1 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @forward_deref(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^join(%left : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %place = "sec.place.deref"(%merged) : (!sec.ref<si32>) -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
  func.func @forward_compare(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^join(%left : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %same = "sec.ref.compare"(%merged, %merged) <{predicate = "eq"}> : (!sec.ref<si32>, !sec.ref<si32>) -> i1
    return
  }
}

// -----

module {
  func.func @forward_reborrow(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^join(%left : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %child = "sec.ref.reborrow_shared"(%merged) <{borrow_id = 2 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, source_borrow = 1 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @loop_deref(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^join(%left : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %place = "sec.place.deref"(%merged) : (!sec.ref<si32>) -> !sec.place<si32, "ro">
    cf.cond_br %flag, ^join(%merged : !sec.ref<si32>), ^exit
  ^exit:
    return
  }
}

// -----

module {
  func.func @loop_compare(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^join(%left : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %same = "sec.ref.compare"(%merged, %merged) <{predicate = "eq"}> : (!sec.ref<si32>, !sec.ref<si32>) -> i1
    cf.cond_br %flag, ^join(%merged : !sec.ref<si32>), ^exit
  ^exit:
    return
  }
}

// -----

module {
  func.func @loop_reborrow(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.br ^join(%left : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %child = "sec.ref.reborrow_shared"(%merged) <{borrow_id = 2 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, source_borrow = 1 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    cf.cond_br %flag, ^join(%merged : !sec.ref<si32>), ^exit
  ^exit:
    return
  }
}

// -----

module {
  func.func @wrong_source(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, address_stability = "Stable", provenance = "local", validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    %valid = "sec.ref.is_valid"(%left) : (!sec.ref<si32>) -> i1
    cf.cond_br %valid, ^live, ^stale
  ^live:
    // expected-error@+1 {{use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge}}
    %place = "sec.place.deref"(%merged) : (!sec.ref<si32>) -> !sec.place<si32, "ro">
    return
  ^stale:
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
}
