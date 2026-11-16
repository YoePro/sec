// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// SVV-2: a static union, Option or Result value names its active variant and
// carries exactly the payload that variant declares; enum and string values
// carry their case or text (compile_time_evaluation.md § 32(3)).

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "s1", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.static_initializer = {payload = "9", variant = "Number"}, sec.source_name = "s", sec.static_id = 1 : i32} : () -> ()
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "s2", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.static_initializer = {variant = "Empty"}, sec.source_name = "s", sec.static_id = 2 : i32} : () -> ()
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "s3", type = !sec.result<none, !sec.int>}> {sec.static_initializer = {variant = "Ok"}, sec.source_name = "s", sec.static_id = 3 : i32} : () -> ()
  "sec.static.global"() <{initial = "string", is_mutable = false, sym_name = "s4", type = !sec.string}> {sec.static_initializer = {string = "Sec"}, sec.source_name = "s", sec.static_id = 4 : i32} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{union static initializer names unknown variant Missing}}
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "s1", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.static_initializer = {variant = "Missing"}, sec.source_name = "s", sec.static_id = 1 : i32} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{variant Number static initializer requires its payload}}
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "s1", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.static_initializer = {variant = "Number"}, sec.source_name = "s", sec.static_id = 1 : i32} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{variant Empty has no payload}}
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "s1", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.static_initializer = {payload = "1", variant = "Empty"}, sec.source_name = "s", sec.static_id = 1 : i32} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{variant Ok has no payload}}
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "s1", type = !sec.result<none, !sec.int>}> {sec.static_initializer = {payload = "1", variant = "Ok"}, sec.source_name = "s", sec.static_id = 1 : i32} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{requires initial = "variant"}}
  "sec.static.global"() <{initial = "aggregate", is_mutable = false, sym_name = "s1", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.static_initializer = {variant = "Empty"}, sec.source_name = "s", sec.static_id = 1 : i32} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{string static initializer carries exactly its text}}
  "sec.static.global"() <{initial = "string", is_mutable = false, sym_name = "s1", type = !sec.string}> {sec.static_initializer = {string = "a", extra = "b"}, sec.source_name = "s", sec.static_id = 1 : i32} : () -> ()
}
