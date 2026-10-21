// RUN: sec-mlir-opt %s --sec-verify-slice-guards -split-input-file -verify-diagnostics -o /dev/null
// P16-42: malformed CFGs retain the owning slice guard diagnostic.

// reversed outcomes
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^ok, ^bad
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok:
  // expected-error @+1 {{slice range result used outside its guarded edge}}
  return %n : !sec.uint
}
}

// -----

// error used on success
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok:
  // expected-error @+1 {{slice range result used outside its guarded edge}}
  %discarded = "sec.enum.cmp"(%error, %error) <{predicate = "eq"}> : (!R, !R) -> i1
  return %n : !sec.uint
}
}

// -----

// missing canonical failed branch
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  // expected-error @+1 {{range check requires an immediate single-use failed branch}}
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  %unused = arith.constant true
  cf.cond_br %failed, ^bad, ^ok
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok:
  return %n : !sec.uint
}
}

// -----

// wrong range endpoint
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  // expected-error @+1 {{range failure requires slice-range bounds provenance}}
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
^ok:
  return %n : !sec.uint
}
}

// -----

// lost RangeError flow
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  // expected-error @+1 {{range failure must panic or consume canonical RangeError}}
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  return %total : !sec.uint
^ok:
  return %n : !sec.uint
}
}

// -----

// wrong tuple order
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @reborrow(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  %total = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  // expected-error @+1 {{construction requires the same normalized start/length tuple}}
  %child = "sec.slice.reborrow_shared"(%view, %n, %s) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
  return
}
}

// -----

// wrong source length
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @reborrow(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) {
  %total = "sec.slice.len"(%other) : (!sec.slice<si128>) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  // expected-error @+1 {{reborrow range must use the same source slice length}}
  %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
  return
}
}

// -----

// wrong array extent
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @borrow(%array: !sec.place<!sec.array<si128, "4">, "ro">) {
  %total = "sec.const.int"() <{value = "3"}> : () -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  // expected-error @+1 {{fixed-array range requires its exact source extent}}
  %child = "sec.slice.borrow_shared"(%array, %s, %n) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}> : (!sec.place<!sec.array<si128, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.slice<si128>
  return
}
}

// -----

// unrelated index
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  // expected-error @+1 {{element Place requires the same slice/index bounds-true edge}}
  %place = "sec.slice.element_place"(%view, %otherIndex) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}

// -----

// unrelated slice
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  // expected-error @+1 {{element Place requires the same slice/index bounds-true edge}}
  %place = "sec.slice.element_place"(%other, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}

// -----

// access on false edge
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  // expected-error @+1 {{index guard requires true access and a dedicated bounds/IndexError failure edge}}
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^bad, ^access
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}

// -----

// wrong index panic provenance
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  // expected-error @+1 {{index guard requires true access and a dedicated bounds/IndexError failure edge}}
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
}
}

// -----

// index guard fails by return
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  // expected-error @+1 {{index guard requires true access and a dedicated bounds/IndexError failure edge}}
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  %zero = "sec.const.int"() <{value = "0"}> : () -> si128
  return %zero : si128
}
}

// -----

// length validates another slice
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @length(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  %valid = "sec.ref.is_valid"(%other) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  // expected-error @+1 {{dynamic slice observation requires a dominating same-source validity guard}}
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
}

// -----

// stale failure becomes spatial
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @length(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  // expected-error @+1 {{dynamic slice observation requires a dominating same-source validity guard}}
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}

// -----

// missing temporal guard on index
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> si128 {
  // expected-error @+1 {{dynamic slice observation requires a dominating same-source validity guard}}
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}

// -----

// missing reference facts
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @length(%view: !sec.slice<si128>, %other: !sec.slice<si128>) -> !sec.uint {
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^ok, ^stale
^ok:
  // expected-error @+1 {{slice observation requires canonical P15 reference facts}}
  %length = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  return %length : !sec.uint
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
}

// -----

// forwarded on wrong edge
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @forward(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  // expected-error @+1 {{slice range result forwarded on the wrong outcome}}
  cf.cond_br %failed, ^bad(%n : !sec.uint), ^ok
^bad(%badCount: !sec.uint):
  "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
^ok:
  return %total : !sec.uint
}
}

// -----

// range success bypass
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @range(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  cf.br ^check
^check:
  // expected-error @+1 {{range outcomes require dedicated successors}}
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  cf.br ^ok
^ok:
  return %n : !sec.uint
}
}

// -----

// A nominal spelling is insufficient without canonical IndexError cases.
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "7">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @fallible_index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256) -> !sec.result<si128, !I> {
  // expected-error @+1 {{index guard requires true access and a dedicated bounds/IndexError failure edge}}
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
}

// -----

!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @index(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: si256, %otherIndex: si256, %skip: i1) -> si128 {
  cf.cond_br %skip, ^access, ^test
^test:
  %ok = "sec.slice.index_in_bounds"(%view, %index) <{index_signed = true}> : (!sec.slice<si128>, si256) -> i1
  cf.cond_br %ok, ^access, ^bad
^access:
  // expected-error @+1 {{element Place requires the same slice/index bounds-true edge}}
  %place = "sec.slice.element_place"(%view, %index) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}

// -----

!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @dynamic_range(%view: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}, %other: !sec.slice<si128> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter"}}) {
  // expected-error @+1 {{dynamic slice observation requires a dominating same-source validity guard}}
  %total = "sec.slice.len"(%view) : (!sec.slice<si128>) -> !sec.uint
  %valid = "sec.ref.is_valid"(%view) : (!sec.slice<si128>) -> i1
  cf.cond_br %valid, ^spatial, ^stale
^spatial:
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  %child = "sec.slice.reborrow_shared"(%view, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 7 : i32, epoch_dependency = 9 : i32, address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.slice<si128>, !sec.uint, !sec.uint) -> !sec.slice<si128>
  "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.slice<si128>) -> ()
  return
^stale:
  "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
}
}

// -----

!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @forward(%total: !sec.uint, %start: si128, %end: ui256) -> !sec.uint {
  %s, %n, %failed, %error = "sec.slice.range_check"(%total, %start, %end) <{start_present = true, end_present = true, end_exclusive = false, start_signed = true, end_signed = false}> : (!sec.uint, si128, ui256) -> (!sec.uint, !sec.uint, i1, !R)
  cf.cond_br %failed, ^bad, ^ok(%n : !sec.uint)
^bad:
  cf.br ^merge(%total : !sec.uint)
^ok(%count: !sec.uint):
  cf.br ^merge(%count : !sec.uint)
^merge(%joined: !sec.uint):
  // expected-error @+1 {{slice range result used outside its guarded edge}}
  return %joined : !sec.uint
}
}

// -----

// Two unknown phi origins never establish equality.
!R = !sec.enum<identity = "core::RangeError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>
!I = !sec.enum<identity = "core::IndexError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfBounds", value = "0">]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-guards", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
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
  // expected-error @+1 {{element Place requires the same slice/index bounds-true edge}}
  %place = "sec.slice.element_place"(%view, %actual) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
  %value = "sec.place.read"(%place) <{action = "copy-trivial"}> : (!sec.place<si128, "ro">) -> si128
  return %value : si128
^bad:
  "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
}
}
