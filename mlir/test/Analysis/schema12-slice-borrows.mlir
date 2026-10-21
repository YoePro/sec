// RUN: sec-mlir-opt %s --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | sec-mlir-opt --sec-verify-borrow-semantics | FileCheck %s
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md §§39–46,60–63,83–86,116.
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-borrows", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @independent_shared_copy(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    %length = "sec.slice.len"(%copy) : (!sec.slice<si32>) -> !sec.uint
    "sec.ref.end_borrow"(%copy) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    return
  }
  func.func @move_destination(%view: !sec.slice_mut<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %copy = "sec.slice.move_mut"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si32>) -> !sec.slice_mut<si32>
    %length = "sec.slice.len"(%copy) : (!sec.slice_mut<si32>) -> !sec.uint
    "sec.ref.end_borrow"(%copy) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice_mut<si32>) -> ()
    return
  }
  func.func @dynamic_transfers_do_not_observe(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    "sec.ref.end_borrow"(%copy) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    return
  }
  func.func @parent_after_child_end(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
    %total = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si32>) -> ()
    %again = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    return
  }
  func.func @returned_slice(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    return %view : !sec.slice<si32>
  }
  func.func @returned_child(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %total = "sec.slice.len"(%view) : (!sec.slice<si32>) -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>, !sec.uint, !sec.uint) -> !sec.slice<si32>
    cf.br ^exit(%child : !sec.slice<si32>)
  ^exit(%result: !sec.slice<si32>):
    return %result : !sec.slice<si32>
  }
  func.func @fresh_loop_instance(%view: !sec.slice<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %again: i1) {
    cf.br ^loop
    ^loop:
    %copy = "sec.slice.copy_shared"(%view) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si32>) -> !sec.slice<si32>
    "sec.ref.end_borrow"(%copy) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.slice<si32>) -> ()
    cf.cond_br %again, ^loop, ^done
    ^done:
    return
  }
  func.func @returned_array_view(%reference: !sec.ref<!sec.array<si32, "4">> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]} {
    %place = "sec.place.deref"(%reference) : (!sec.ref<!sec.array<si32, "4">>) -> !sec.place<!sec.array<si32, "4">, "ro">
    %four = "sec.const.int"() <{value = "4"}> : () -> !sec.uint
    %s, %n = "sec.slice.range_normalize"(%four) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %view = "sec.slice.borrow_shared"(%place, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.array<si32, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si32>
    return %view : !sec.slice<si32>
  }

}
// CHECK-LABEL: func.func @independent_shared_copy(
// CHECK-LABEL: func.func @move_destination(
// CHECK-LABEL: func.func @dynamic_transfers_do_not_observe(
// CHECK-LABEL: func.func @parent_after_child_end(
// CHECK-LABEL: func.func @returned_slice(
// CHECK-LABEL: func.func @returned_child(
// CHECK-LABEL: func.func @fresh_loop_instance(
// CHECK-LABEL: func.func @returned_array_view(
