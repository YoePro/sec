// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-126: malformed StringView operations are rejected by their op verifiers.
// Rules: rules/collections/collections.md 7.10; rules/control-flow/flowcontrol_for.md 19.

// view over a non-string Place
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "string-views", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @f(%p: !sec.place<!sec.array<si32, "4">, "ro">, %s: !sec.uint, %n: !sec.uint) {
  // expected-error @+1 {{string view source must be a string Place}}
  %v = "sec.string.view_borrow"(%p, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.array<si32, "4">, "ro">, !sec.uint, !sec.uint) -> !sec.string_view
  return
}
}

// -----

// rune count of a non-string Place
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "string-views", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @f(%p: !sec.place<si32, "ro">) {
  // expected-error @+1 {{rune length requires a string Place and uint count result}}
  %n = "sec.string.rune_len"(%p) : (!sec.place<si32, "ro">) -> !sec.uint
  return
}
}

// -----

// a view borrow cannot produce a rune slice
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "string-views", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @f(%p: !sec.place<!sec.string, "ro">, %s: !sec.uint, %n: !sec.uint) {
  // expected-error @+1 {{string view borrow must produce !sec.string_view}}
  %v = "sec.string.view_borrow"(%p, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.string, "ro">, !sec.uint, !sec.uint) -> !sec.slice<!sec.rune>
  return
}
}

// -----

// a view reborrow stays a shared StringView
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "string-views", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @f(%v: !sec.string_view, %s: !sec.uint, %n: !sec.uint) {
  // expected-error @+1 {{a StringView reborrow must produce a shared StringView}}
  %c = "sec.slice.reborrow_shared"(%v, %s, %n) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.string_view, !sec.uint, !sec.uint) -> !sec.slice<!sec.rune>
  return
}
}

// -----

// decoding yields a rune by value
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "string-views", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @f(%v: !sec.string_view, %c: !sec.uint) {
  // expected-error @+1 {{decode_next requires a StringView, a uint cursor and a rune result}}
  %has, %ch, %next = "sec.string_view.decode_next"(%v, %c) : (!sec.string_view, !sec.uint) -> (i1, !sec.char, !sec.uint)
  return
}
}

// -----

// byte count of a non-view
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "string-views", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
func.func @f(%v: !sec.slice<si32>) {
  // expected-error @+1 {{byte_len requires a StringView and uint count result}}
  %n = "sec.string_view.byte_len"(%v) : (!sec.slice<si32>) -> !sec.uint
  return
}
}
