// RUN: sec-mlir-opt %s -split-input-file --sec-verify-places -verify-diagnostics

// SVV-3: a payload Place of a static is proven by the active-variant test of
// a static.load of the same global; the test of another static proves nothing.

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.source_files = []} {
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "sec_static_1", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.address_taken = true, sec.static_initializer = {payload = "1", variant = "Number"}, sec.source_name = "main::S1", sec.static_id = 1 : i32} : () -> ()
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "sec_static_2", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.address_taken = true, sec.static_initializer = {payload = "2", variant = "Number"}, sec.source_name = "main::S2", sec.static_id = 2 : i32} : () -> ()
  func.func @read() -> !sec.int {
    %v0 = "sec.static.load"() <{global = @sec_static_1}> : () -> !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>
    %v1 = "sec.union.is_variant"(%v0) <{variant = 1 : i32}> : (!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>) -> i1
    cf.cond_br %v1, ^bb1, ^bb2
  ^bb1:
    %v2 = "sec.place.static"() <{global = @sec_static_1, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>, "ro">
    %v3 = "sec.place.union_payload"(%v2) <{variant = 1 : i32}> : (!sec.place<!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
    %v4 = "sec.place.read"(%v3) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
    return %v4 : !sec.int
  ^bb2:
    %v5 = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    return %v5 : !sec.int
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.source_files = []} {
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "sec_static_1", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.address_taken = true, sec.static_initializer = {payload = "1", variant = "Number"}, sec.source_name = "main::S1", sec.static_id = 1 : i32} : () -> ()
  "sec.static.global"() <{initial = "variant", is_mutable = false, sym_name = "sec_static_2", type = !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>}> {sec.address_taken = true, sec.static_initializer = {payload = "2", variant = "Number"}, sec.source_name = "main::S2", sec.static_id = 2 : i32} : () -> ()
  func.func @read() -> !sec.int {
    %v0 = "sec.static.load"() <{global = @sec_static_1}> : () -> !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>
    %v1 = "sec.union.is_variant"(%v0) <{variant = 1 : i32}> : (!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>) -> i1
    cf.cond_br %v1, ^bb1, ^bb2
  ^bb1:
    %v2 = "sec.place.static"() <{global = @sec_static_2, place_root_id = 2 : i32, storage_identity = 2 : i32, address_space = "ordinary"}> : () -> !sec.place<!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>, "ro">
    // expected-error@+1 {{payload Place requires a dominating active-variant test of the same Place}}
    %v3 = "sec.place.union_payload"(%v2) <{variant = 1 : i32}> : (!sec.place<!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Empty", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Number", kind = "single", payload = !sec.int, fields = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
    %v4 = "sec.place.read"(%v3) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "ro">) -> !sec.int
    return %v4 : !sec.int
  ^bb2:
    %v5 = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    return %v5 : !sec.int
  }
}
