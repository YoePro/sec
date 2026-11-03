// RUN: sec-mlir-opt %s -split-input-file --sec-verify-ownership -verify-diagnostics
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 9-18, 111-114.

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h) {
    "sec.own.discard"(%v) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h, %c: i1) {
    cf.cond_br %c, ^bb1, ^bb2
    ^bb1:
    "sec.own.discard"(%v) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    cf.br ^bb2
    ^bb2:
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h, %c: i1) {
    cf.br ^bb1
    ^bb1:
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    cf.cond_br %c, ^bb1, ^bb2
    ^bb2:
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h) {
    // expected-error@+1 {{copies a value that needs destruction}}
    %c = "sec.own.copy"(%v) : (!h) -> !h
    "sec.call.direct"(%c) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !sec.place<!h, "rw">) {
    // expected-error@+1 {{copies a value that needs destruction}}
    %c = "sec.own.copy_from_place"(%a) <{copy_mode = "trivial"}> : (!sec.place<!h, "rw">) -> !h
    "sec.call.direct"(%c) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !sec.place<!h, "rw">) {
    // expected-error@+1 {{copies a value that needs destruction}}
    %c = "sec.place.read"(%a) <{action = "copy-trivial"}> : (!sec.place<!h, "rw">) -> !h
    "sec.call.direct"(%c) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !h) {
    // expected-error@+1 {{copies a value that needs destruction}}
    "sec.call.direct"(%a) <{callee = @sink}> {sec.argument_actions = ["copy-trivial"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad() {
    // expected-error@+2 {{leaves an owned value with no terminal ownership action}}
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%c: i1) {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    cf.cond_br %c, ^bb1, ^bb2
    ^bb1:
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^bb2:
    // expected-error@+1 {{leaves an owned value with no terminal ownership action}}
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h) {
    // expected-error@+1 {{leaves an owned value with no terminal ownership action}}
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%c: i1) {
    cf.br ^bb1
    ^bb1:
    // expected-error@+1 {{redefines a value that was never destroyed or transferred}}
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    cf.cond_br %c, ^bb1, ^bb2
    ^bb2:
    "sec.own.discard"(%v) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad() {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "h"}> : (!h) -> ()
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    // expected-error@+1 {{leaves an owned value with no terminal ownership action}}
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad() {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "h"}> : (!h) -> ()
    // expected-error@+1 {{transfers a value still registered for cleanup without sec.own.move}}
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%c: i1) {
    cf.br ^bb1
    ^bb1:
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "h"}> : (!h) -> ()
    "sec.cleanup.run_scope"() <{scope_ids = array<i32: 1>, exit_kind = "fallthrough"}> : () -> ()
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h) {
    // expected-error@+1 {{transfers one owner twice along one edge}}
    cf.br ^bb1(%v, %v : !h, !h) {sec.successor_actions = [["move", "move"]]}
    ^bb1(%x: !h, %y: !h):
    "sec.own.discard"(%x) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    "sec.own.discard"(%y) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!h>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    %o = "sec.own.move_from_place"(%p0) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!h>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    "sec.own.initialize_place"(%p0, %v) : (!sec.place<!h, "rw">, !h) -> ()
    %n = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    // expected-error@+1 {{initializes a Place that may still own a value}}
    "sec.own.initialize_place"(%p0, %n) : (!sec.place<!h, "rw">, !h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h, %n: !h, %c: i1) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!h>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    "sec.own.initialize_place"(%p0, %v) : (!sec.place<!h, "rw">, !h) -> ()
    cf.cond_br %c, ^bb1, ^bb2
    ^bb1:
    %o = "sec.own.move_from_place"(%p0) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    cf.br ^bb2
    ^bb2:
    // expected-error@+1 {{initializes a Place that may still own a value}}
    "sec.own.initialize_place"(%p0, %n) : (!sec.place<!h, "rw">, !h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%v: !h, %n: !h) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!h>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!h>) -> !sec.place<!h, "rw">
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    "sec.own.replace_place"(%p0, %n) <{destruction_plan = "h", old_cleanup_action_id = 1 : i32, new_cleanup_action_id = 2 : i32}> : (!sec.place<!h, "rw">, !h) -> ()
    "sec.own.discard"(%v) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p0, %a) : (!sec.place<!p, "rw">, !p) -> ()
    %f = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %o = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    %w = "sec.own.move_from_place"(%p0) : (!sec.place<!p, "rw">) -> !p
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p0, %a) : (!sec.place<!p, "rw">, !p) -> ()
    %f = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %o = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    %g = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    %w = "sec.own.move_from_place"(%g) : (!sec.place<!h, "rw">) -> !h
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p0, %a) : (!sec.place<!p, "rw">, !p) -> ()
    %f = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %o = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    // expected-error@+1 {{destroys moved subobject 'field#2' without skipping it}}
    "sec.own.destroy_place"(%p0) <{destruction_plan = "p", cause = "function-exit"}> : (!sec.place<!p, "rw">) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p0, %a) : (!sec.place<!p, "rw">, !p) -> ()
    // expected-error@+1 {{skips subobject 'field#2', which is available on some path}}
    "sec.own.destroy_place"(%p0) <{destruction_plan = "p", cause = "function-exit", skipped = ["field#2"]}> : (!sec.place<!p, "rw">) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "s", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p0 = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p0, %a) : (!sec.place<!p, "rw">, !p) -> ()
    "sec.own.destroy_place"(%p0) <{destruction_plan = "p", cause = "function-exit"}> : (!sec.place<!p, "rw">) -> ()
    %f = "sec.place.field"(%p0) <{field = 2 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %n = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    // expected-error@+1 {{initializes a subobject of an unavailable value}}
    "sec.own.initialize_place"(%f, %n) : (!sec.place<!h, "rw">, !h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%r: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    %d = "sec.place.deref"(%r) : (!sec.ref_mut<!h>) -> !sec.place<!h, "rw">
    // expected-error@+1 {{moves out of a borrowed referent}}
    %o = "sec.own.move_from_place"(%d) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%w: !sec.place<!sec.array<!h, "2">, "rw">, %i: !sec.int) {
    %e = "sec.place.array_element"(%w, %i) <{bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.place<!sec.array<!h, "2">, "rw">, !sec.int) -> !sec.place<!h, "rw">
    // expected-error@+1 {{moves out of an array element}}
    %o = "sec.own.move_from_place"(%e) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "cf", type = !CF, kind = "struct", custom_free = true, free_type = "CF", free_function = @cffree, fields = [{field = 0 : i32, plan = "h"}]}, {id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func @cffree(%self: !sec.ref_mut<!CF> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @bad(%a: !sec.place<!CF, "rw">) {
    %f = "sec.place.field"(%a) <{field = 0 : i32}> : (!sec.place<!CF, "rw">) -> !sec.place<!h, "rw">
    // expected-error@+1 {{moves a field out of a value with custom free}}
    %o = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    "sec.own.discard"(%o) <{destruction_plan = "h", discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @result_without_action() {
    // expected-error@+1 {{the call result needs destruction, so its ownership action must be "move"}}
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @return_copied() -> !h {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    // expected-error@+1 {{the returned value needs destruction, so its ownership action must be "move"}}
    "func.return"(%v) {sec.return_action = "copy"} : (!h) -> ()
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @branch_without_action(%c: i1) {
    %v = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = [], sec.result_action = "move"} : () -> !h
    // expected-error@+1 {{a forwarded branch operand needs destruction, so its ownership action must be "move"}}
    cf.br ^bb1(%v : !h)
    ^bb1(%x: !h):
    "sec.call.direct"(%x) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @non_owning_value() -> !sec.int {
    %v = "sec.const.int"() <{value = "1"}> : () -> !sec.int
    // expected-error@+1 {{is not a reference or slice holder, so it cannot be "non-owning"}}
    "func.return"(%v) {sec.return_action = "non-owning"} : (!sec.int) -> ()
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!q = !sec.struct<identity = "main::Q", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "a", type = !sec.struct<identity = "main::T", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "x", type = !sec.int, tags = []>]>, tags = []>]>
!e = !sec.result<!h, !sec.int>
!CF = !sec.struct<identity = "main::CF", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "h", type = !h, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "e", type = !e, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func @destroy_uninitialized_field(%v: !p) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "p", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!p>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!p>) -> !sec.place<!p, "rw">
    "sec.own.initialize_place"(%p, %v) : (!sec.place<!p, "rw">, !p) -> ()
    %f = "sec.place.field"(%p) <{field = 0 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    %m = "sec.own.move_from_place"(%f) : (!sec.place<!h, "rw">) -> !h
    "sec.call.direct"(%m) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    // expected-error@+1 {{destroys a value that is moved or uninitialized on some path}}
    "sec.own.destroy_place"(%f) <{destruction_plan = "h", cause = "scope-exit"}> : (!sec.place<!h, "rw">) -> ()
    "sec.own.destroy_place"(%p) <{destruction_plan = "p", cause = "scope-exit", skipped = ["field#0"]}> : (!sec.place<!p, "rw">) -> ()
    return
  }
}
