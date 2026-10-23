// RUN: sec-mlir-opt %s --sec-verify-borrow-semantics -split-input-file -verify-diagnostics -o /dev/null
// Schema 11 sections 20,25,37: forwarded holders retain lifetime obligations.
module {
  func.func @alias(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.br ^next(%source : !sec.ref<si32>)
  ^next(%forwarded: !sec.ref<si32>):
    "sec.ref.end_borrow"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<si32>) -> ()
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %place = "sec.place.deref"(%forwarded) : (!sec.ref<si32>) -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
  func.func @alias(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.br ^next(%source : !sec.ref<si32>)
  ^next(%forwarded: !sec.ref<si32>):
    "sec.ref.end_borrow"(%forwarded) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<si32>) -> ()
    // expected-error@+1 {{reference used after its sec.ref.end_borrow}}
    %place = "sec.place.deref"(%source) : (!sec.ref<si32>) -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
  func.func @alias(%source: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.br ^next(%source : !sec.ref_mut<si32>)
  ^next(%forwarded: !sec.ref_mut<si32>):
    %moved = "sec.ref.move"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    // expected-error@+1 {{!sec.ref_mut used after it was moved}}
    %place = "sec.place.deref"(%forwarded) : (!sec.ref_mut<si32>) -> !sec.place<si32, "rw">
    return
  }
}

// -----

module {
  func.func @alias(%source: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.br ^next(%source : !sec.ref_mut<si32>)
  ^next(%forwarded: !sec.ref_mut<si32>):
    %moved = "sec.ref.move"(%forwarded) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    // expected-error@+1 {{!sec.ref_mut used after it was moved}}
    %place = "sec.place.deref"(%source) : (!sec.ref_mut<si32>) -> !sec.place<si32, "rw">
    return
  }
}

// -----

module {
  func.func @wrong_end(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.br ^next(%source : !sec.ref<si32>)
  ^next(%forwarded: !sec.ref<si32>):
    // expected-error@+1 {{end_borrow must name the ended reference's borrow_id and lifetime_id}}
    "sec.ref.end_borrow"(%forwarded) <{borrow_id = 2 : i32, lifetime_id = 1 : i32}> : (!sec.ref<si32>) -> ()
    return
  }
}

// -----

module {
  func.func @wrong_end(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.br ^next(%source : !sec.ref<si32>)
  ^next(%forwarded: !sec.ref<si32>):
    // expected-error@+1 {{end_borrow must name the ended reference's borrow_id and lifetime_id}}
    "sec.ref.end_borrow"(%forwarded) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.ref<si32>) -> ()
    return
  }
}

// -----

module {
  func.func @join_end(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 3 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{end_borrow must name the ended reference's borrow_id and lifetime_id}}
    "sec.ref.end_borrow"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<si32>) -> ()
    return
  }
}

// -----

module {
  func.func @join_reborrow(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 3 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    // expected-error@+1 {{source_borrow must name the borrow_id of the reference it derives from}}
    %child = "sec.ref.reborrow_shared"(%merged) <{borrow_id = 2 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}
