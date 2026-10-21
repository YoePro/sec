// RUN: split-file %s %t
// RUN: sec-mlir-opt %t/32.mlir --sec-verify-slice-guards | sec-mlir-opt --sec-verify-slice-guards | FileCheck %s
// RUN: sec-mlir-opt %t/64.mlir --sec-verify-slice-guards | sec-mlir-opt --sec-verify-slice-guards | FileCheck %s
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout --sec-verify-slice-guards -o /dev/null
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout --sec-verify-slice-guards -o /dev/null
// RUN: not sec-mlir-opt %t/bad.mlir --sec-verify-slice-guards -o %t/rejected.mlir 2>&1 | FileCheck %s --check-prefix=REJECTED
// RUN: test ! -e %t/rejected.mlir
// REJECTED: dynamic slice observation requires a dominating same-source validity guard
// P16-42: explicit guard verification, not physical lowering or P43 ownership.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md sections 83,105-110.
// CHECK: func.func @range
// CHECK: func.func @fallible
// CHECK: func.func @forward
// CHECK: func.func @index
// CHECK: func.func @mutable_index
// CHECK: func.func @dynamic_index
// CHECK: func.func @fallible_index
// CHECK: func.func @length
// CHECK: func.func @reborrow
// CHECK: func.func @borrow

//--- 32.mlir
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok:
  return %n : !sec.uint
}
func.func @fallible(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.result<!sec.uint, !R> {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  %err = "sec.result.err"(%error) : (!R) -> !sec.result<!sec.uint, !R>
  return %err : !sec.result<!sec.uint, !R>
^ok:
  %ok = "sec.result.ok"(%n) : (!sec.uint) -> !sec.result<!sec.uint, !R>
  return %ok : !sec.result<!sec.uint, !R>
}
func.func @forward(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok(%n : !sec.uint)
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok(%count: !sec.uint):
  return %count : !sec.uint
}
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @mutable_index(%view: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice_mut<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice_mut<si128>, si256) -> !sec.place<si128, "rw">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "rw">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @dynamic_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^spatial, ^stale
^spatial:
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @fallible_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> !sec.result<si128, !I> {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  %rv = "sec.result.ok"(%value) : (si128) -> !sec.result<si128, !I>
  return %rv : !sec.result<si128, !I>
^bad:
  %error = "sec.enum.constant"() <{caseOrdinal = 0 : i32}> : () -> !I
  %err = "sec.result.err"(%error) : (!I) -> !sec.result<si128, !I>
  return %err : !sec.result<si128, !I>
}
func.func @length(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
func.func @reborrow(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  %total = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
  return
}
func.func @borrow(%array: !sec.place<!sec.array<si128, "4">, "ro">) {
  %total = "sec.const.int"() <{value = "4"}> : () -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.borrow_shared"(%array, %s, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
  return
}
func.func @forwarded_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  cf.br ^forward(%view : !sec.slice<si128>)
^forward(%alias: !sec.slice<si128>):
  %ok = "sec.slice.index_in_bounds"(%alias, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @delayed_failure(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  cf.br ^panic
^panic:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @proven(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "proven-safe", bounds_proof = "contract"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
}
func.func @forwarded_dynamic(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  cf.br ^forward(%view : !sec.slice<si128>)
^forward(%alias: !sec.slice<si128>):
  %valid = "sec.ref.is_valid"(%alias) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
func.func @mutable_reborrow(%view: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  %total = "sec.slice.len"(%view) : (!sec.slice_mut<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.reborrow_mut"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si128>, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice_mut<si128>) -> ()
  return
}
func.func @dynamic_range(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^spatial, ^stale
^spatial:
  %total = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
  return
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
func.func @fallible_forward(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.result<!sec.uint, !R> {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad(%error : !R), ^ok
^bad(%cause: !R):
  %err = "sec.result.err"(%cause) : (!R) -> !sec.result<!sec.uint, !R>
  return %err : !sec.result<!sec.uint, !R>
^ok:
  %ok = "sec.result.ok"(%n) : (!sec.uint) -> !sec.result<!sec.uint, !R>
  return %ok : !sec.result<!sec.uint, !R>
}
func.func @phi_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256, %choice: i1) -> si128 {
  cf.cond_br %choice, ^left, ^right
^left:
  cf.br ^test(%index, %otherIndex : si256, si256)
^right:
  cf.br ^test(%otherIndex, %index : si256, si256)
^test(%tested: si256, %actual: si256):
  %ok = "sec.slice.index_in_bounds"(%view, %tested) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %tested) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}
//--- 64.mlir
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok:
  return %n : !sec.uint
}
func.func @fallible(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.result<!sec.uint, !R> {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  %err = "sec.result.err"(%error) : (!R) -> !sec.result<!sec.uint, !R>
  return %err : !sec.result<!sec.uint, !R>
^ok:
  %ok = "sec.result.ok"(%n) : (!sec.uint) -> !sec.result<!sec.uint, !R>
  return %ok : !sec.result<!sec.uint, !R>
}
func.func @forward(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok(%n : !sec.uint)
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok(%count: !sec.uint):
  return %count : !sec.uint
}
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @mutable_index(%view: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice_mut<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice_mut<si128>, si256) -> !sec.place<si128, "rw">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "rw">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @dynamic_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^spatial, ^stale
^spatial:
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @fallible_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> !sec.result<si128, !I> {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  %rv = "sec.result.ok"(%value) : (si128) -> !sec.result<si128, !I>
  return %rv : !sec.result<si128, !I>
^bad:
  %error = "sec.enum.constant"() <{caseOrdinal = 0 : i32}> : () -> !I
  %err = "sec.result.err"(%error) : (!I) -> !sec.result<si128, !I>
  return %err : !sec.result<si128, !I>
}
func.func @length(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
func.func @reborrow(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  %total = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
  return
}
func.func @borrow(%array: !sec.place<!sec.array<si128, "4">, "ro">) {
  %total = "sec.const.int"() <{value = "4"}> : () -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.borrow_shared"(%array, %s, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
  return
}
func.func @forwarded_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  cf.br ^forward(%view : !sec.slice<si128>)
^forward(%alias: !sec.slice<si128>):
  %ok = "sec.slice.index_in_bounds"(%alias, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @delayed_failure(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  cf.br ^panic
^panic:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
func.func @proven(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "proven-safe", bounds_proof = "contract"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
}
func.func @forwarded_dynamic(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  cf.br ^forward(%view : !sec.slice<si128>)
^forward(%alias: !sec.slice<si128>):
  %valid = "sec.ref.is_valid"(%alias) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
func.func @mutable_reborrow(%view: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice_mut<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  %total = "sec.slice.len"(%view) : (!sec.slice_mut<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.reborrow_mut"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice_mut<si128>, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice_mut<si128>) -> ()
  return
}
func.func @dynamic_range(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^spatial, ^stale
^spatial:
  %total = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
  return
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
func.func @fallible_forward(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.result<!sec.uint, !R> {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad(%error : !R), ^ok
^bad(%cause: !R):
  %err = "sec.result.err"(%cause) : (!R) -> !sec.result<!sec.uint, !R>
  return %err : !sec.result<!sec.uint, !R>
^ok:
  %ok = "sec.result.ok"(%n) : (!sec.uint) -> !sec.result<!sec.uint, !R>
  return %ok : !sec.result<!sec.uint, !R>
}
func.func @phi_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256, %choice: i1) -> si128 {
  cf.cond_br %choice, ^left, ^right
^left:
  cf.br ^test(%index, %otherIndex : si256, si256)
^right:
  cf.br ^test(%otherIndex, %index : si256, si256)
^test(%tested: si256, %actual: si256):
  %ok = "sec.slice.index_in_bounds"(%view, %tested) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %tested) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}
//--- bad.mlir
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @length(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  %valid = "sec.ref.is_valid"(%other) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
}
