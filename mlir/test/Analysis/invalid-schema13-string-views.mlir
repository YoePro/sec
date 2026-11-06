// RUN: sec-mlir-opt %s -split-input-file --sec-verify-slice-guards -verify-diagnostics -o /dev/null
// MD-126: a string range must normalize against the rune extent of the same
// string Place it borrows.
// Rules: rules/collections/collections.md 7.10; rules/compiler/semantic_ir.md 17(9).

// extent of another string
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "string-views", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @f(%a: !sec.ref<!sec.string> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %b: !sec.ref<!sec.string> {sec.reference = {borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) -> !sec.uint {
  %pa = "sec.place.deref"(%a) : (!sec.ref<!sec.string>) -> !sec.place<!sec.string, "ro">
  %pb = "sec.place.deref"(%b) : (!sec.ref<!sec.string>) -> !sec.place<!sec.string, "ro">
  %total = "sec.string.rune_len"(%pb) : (!sec.place<!sec.string, "ro">) -> !sec.uint
  %s, %n = "sec.slice.range_normalize"(%total) <{start_present = false, end_present = false, end_exclusive = true, start_signed = false, end_signed = false, proof_kind = "source-open-bound"}> : (!sec.uint) -> (!sec.uint, !sec.uint)
  // expected-error @+1 {{string range must use the same Place's rune length}}
  %v = "sec.string.view_borrow"(%pa, %s, %n) <{borrow_id = 3 : i32, lifetime_id = 3 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.string, "ro">, !sec.uint, !sec.uint) -> !sec.string_view
  %len = "sec.slice.len"(%v) : (!sec.string_view) -> !sec.uint
  return %len : !sec.uint
}
}
