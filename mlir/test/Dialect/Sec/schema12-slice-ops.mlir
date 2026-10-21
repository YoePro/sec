// RUN: split-file %s %t
// RUN: sec-mlir-opt %t/64.mlir | sec-mlir-opt | FileCheck %s --check-prefix=OPS
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout | FileCheck %s --check-prefix=P32
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout | FileCheck %s --check-prefix=P64
// RUN: not sec-mlir-opt %t/64.mlir --sec-check-lowering-support -o %t/rejected.mlir 2>&1 | FileCheck %s --check-prefix=BLOCKED
// RUN: test ! -e %t/rejected.mlir

// P16-40 local operation contracts only; whole-function guard/borrow
// verification remains P16-42/P16-43. There is no physical slice descriptor.
// OPS: "sec.slice.range_normalize"
// OPS: "sec.slice.borrow_shared"
// OPS: "sec.slice.copy_shared"
// OPS: "sec.slice.reborrow_shared"
// OPS: "sec.slice.len"
// OPS: "sec.slice.index_in_bounds"
// OPS: "sec.slice.element_place"
// OPS: "sec.slice.borrow_mut"
// OPS: "sec.slice.reborrow_mut"
// OPS: "sec.slice.move_mut"
// OPS: "sec.slice.range_check"
// P32: "sec.slice.len"{{.*}} -> ui32
// P32: "sec.slice.range_check"{{.*}}(ui32, si128, ui256) -> (ui32, ui32, i1,
// P64: "sec.slice.len"{{.*}} -> ui64
// P64: "sec.slice.range_check"{{.*}}(ui64, si128, ui256) -> (ui64, ui64, i1,
// BLOCKED: the Sec MLIR -> LLVM track does not lower 'sec.slice.range_normalize' yet
// BLOCKED: the Sec MLIR -> LLVM track does not lower 'sec.slice.borrow_shared' yet
// BLOCKED: the Sec MLIR -> LLVM track does not lower 'sec.slice.range_check' yet

//--- 32.mlir

!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @shared(%array: !sec.place<!sec.array<si128, "4">, "ro">, %total: !sec.uint, %index: si256) -> !sec.uint {
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %a = "sec.slice.borrow_shared"(%array, %s, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.origin_storages = [7 : i32], sec.slice_static_start = "0", sec.slice_static_end_exclusive = "4"} : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    %b = "sec.slice.copy_shared"(%a) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>) -> !sec.slice<si128>
    %child = "sec.slice.reborrow_shared"(%b, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
    %len = "sec.slice.len"(%child) : (!sec.slice<si128>) -> !sec.uint
    %ok = "sec.slice.index_in_bounds"(%child, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
    cf.cond_br %ok, ^access, ^bad
  ^access:
    %place = "sec.slice.element_place"(%child, %index) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32, index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> {sec.origin_storages = [7 : i32]} : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
    %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
    "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
    return %len : !sec.uint
  ^bad:
    "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
  }
  func.func @mutable(%array: !sec.place<!sec.array<si128, "4">, "rw">, %s: !sec.uint, %n: !sec.uint) {
    %a = "sec.slice.borrow_mut"(%array, %s, %n) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.place<!sec.array<si128, "4">, "rw">, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
    %b = "sec.slice.reborrow_mut"(%a, %s, %n) <{borrow_id = 4 : i32, lifetime_id = 4 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.slice_mut<si128>, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
    "sec.ref.end_borrow"(%b) <{borrow_id = 4 : i32, lifetime_id = 4 : i32}> : (!sec.slice_mut<si128>) -> ()
    %moved = "sec.slice.move_mut"(%a) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.slice_mut<si128>) -> !sec.slice_mut<si128>
    return
  }
  func.func @checked(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
    %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
  ^ok:
    return %n : !sec.uint
  }
}
//--- 64.mlir

!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-64", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @shared(%array: !sec.place<!sec.array<si128, "4">, "ro">, %total: !sec.uint, %index: si256) -> !sec.uint {
    %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %a = "sec.slice.borrow_shared"(%array, %s, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> {sec.origin_storages = [7 : i32], sec.slice_static_start = "0", sec.slice_static_end_exclusive = "4"} : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
    %b = "sec.slice.copy_shared"(%a) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>) -> !sec.slice<si128>
    %child = "sec.slice.reborrow_shared"(%b, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
    %len = "sec.slice.len"(%child) : (!sec.slice<si128>) -> !sec.uint
    %ok = "sec.slice.index_in_bounds"(%child, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
    cf.cond_br %ok, ^access, ^bad
  ^access:
    %place = "sec.slice.element_place"(%child, %index) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32, index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> {sec.origin_storages = [7 : i32]} : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
    %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
    "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
    return %len : !sec.uint
  ^bad:
    "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
  }
  func.func @mutable(%array: !sec.place<!sec.array<si128, "4">, "rw">, %s: !sec.uint, %n: !sec.uint) {
    %a = "sec.slice.borrow_mut"(%array, %s, %n) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.place<!sec.array<si128, "4">, "rw">, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
    %b = "sec.slice.reborrow_mut"(%a, %s, %n) <{borrow_id = 4 : i32, lifetime_id = 4 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.slice_mut<si128>, !sec.uint, !sec.uint) -> !sec.slice_mut<si128>
    "sec.ref.end_borrow"(%b) <{borrow_id = 4 : i32, lifetime_id = 4 : i32}> : (!sec.slice_mut<si128>) -> ()
    %moved = "sec.slice.move_mut"(%a) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 3 : i32}> : (!sec.slice_mut<si128>) -> !sec.slice_mut<si128>
    return
  }
  func.func @checked(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
    %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
  ^ok:
    return %n : !sec.uint
  }
}
