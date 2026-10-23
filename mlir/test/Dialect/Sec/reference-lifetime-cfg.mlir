// RUN: sec-mlir-opt %s --sec-verify-borrow-semantics -split-input-file -verify-diagnostics -o /dev/null
// Schema 11 sections 20,25,37: forwarded holders retain lifetime obligations.
module {
  func.func @join_end(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    "sec.ref.end_borrow"(%merged) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<si32>) -> ()
    return
  }
}

// -----

module {
  func.func @join_reborrow(%flag: i1, %left: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}, %right: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    cf.cond_br %flag, ^left, ^right
  ^left:
    cf.br ^join(%left : !sec.ref<si32>)
  ^right:
    cf.br ^join(%right : !sec.ref<si32>)
  ^join(%merged: !sec.ref<si32>):
    %child = "sec.ref.reborrow_shared"(%merged) <{borrow_id = 2 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref<si32>) -> !sec.ref<si32>
    return
  }
}

// -----

module {
  func.func @shared_copy_survives(%source: !sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    %copy = "sec.ref.copy_shared"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref<si32>) -> !sec.ref<si32>
    "sec.ref.end_borrow"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<si32>) -> ()
    %place = "sec.place.deref"(%copy) : (!sec.ref<si32>) -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
  func.func @moved_result_survives(%source: !sec.ref_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}}) {
    %moved = "sec.ref.move"(%source) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<si32>) -> !sec.ref_mut<si32>
    %place = "sec.place.deref"(%moved) : (!sec.ref_mut<si32>) -> !sec.place<si32, "rw">
    return
  }
}
