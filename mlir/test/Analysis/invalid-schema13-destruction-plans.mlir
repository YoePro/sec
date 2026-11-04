// RUN: sec-mlir-opt %s -split-input-file --sec-verify-destruction-plans -verify-diagnostics
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 116, 123.

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{requires an id, a type and a kind}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", kind = "trivial"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{duplicate destruction plan 'x'}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !sec.int, kind = "trivial"}, {id = "x", type = !sec.char, kind = "trivial"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{is a second plan for}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !sec.int, kind = "trivial"}, {id = "y", type = !sec.int, kind = "trivial"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{has unsupported kind 'unsupported'}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !sec.int, kind = "unsupported"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{carries steps of another kind}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !sec.int, kind = "trivial", element = "y"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{has custom free, which belongs to a struct or union plan}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !sec.int, kind = "trivial", custom_free = true}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'p' is trivial for a type that needs destruction}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "trivial"}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'x' destroys a non-struct}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !sec.int, kind = "struct"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'p' must list existing fields once in reverse declaration order}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 0 : i32, plan = "h"}, {field = 2 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'p' must list existing fields once in reverse declaration order}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 2 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'p' omits non-trivial field #0}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'p' steps to an unknown plan}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "gone"}, {field = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{steps to 'n', which does not destroy}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "n", type = !sec.int, kind = "trivial"}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "n"}, {field = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{has no custom free and no field steps}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !h, kind = "struct"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{names a free function without custom free}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !h, kind = "struct", free_function = @free}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{has custom free without its source type and function}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{names an unknown free function @gone}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @gone}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{needs a free function over a mutable reference to its type}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @bad}]} {
  func.func @bad(%x: !sec.int) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{needs a fixed-array type and its exact element count}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "a", type = !a, kind = "fixed-array", element = "h", element_count = "999"}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'a' steps to an unknown plan}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "a", type = !a, kind = "fixed-array", element_count = "1000000"}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'x' destroys neither a union nor a Result}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !sec.int, kind = "active-variant"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'o' omits non-trivial variant #1}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "o", type = !o, kind = "active-variant"}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{names an invalid, repeated or payload-free variant}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}, {variant = 1 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{names an invalid, repeated or payload-free variant}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}, {variant = 1 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'r' omits non-trivial variant #0}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "r", type = !r, kind = "active-variant"}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{destruction plan 'f' omits non-trivial variant #0}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant"}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{which is not the named-field variant's payload struct}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
// expected-error@+1 {{needs a struct type and a named release contract}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "x", type = !h, kind = "compiler-known-resource"}]} {
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func @use(%v: !h) {
    // expected-error@+1 {{names unknown destruction plan 'gone'}}
    "sec.own.destroy_value"(%v) <{destruction_plan = "gone", cause = "function-exit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "n", type = !sec.int, kind = "trivial"}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func @use(%v: !h) {
    // expected-error@+1 {{names destruction plan 'n', which does not destroy}}
    "sec.own.destroy_value"(%v) <{destruction_plan = "n", cause = "function-exit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func @use(%v: !h) {
    // expected-error@+1 {{discards a value that needs destruction without its plan}}
    "sec.own.discard"(%v) <{discard_origin = "explicit"}> : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func @use(%v: !sec.place<!h, "rw">) {
    // expected-error@+1 {{skips subobjects through plan 'h', which cannot skip}}
    "sec.own.destroy_place"(%v) <{destruction_plan = "h", cause = "function-exit", skipped = ["field#0"]}> : (!sec.place<!h, "rw">) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "p", type = !p, kind = "struct", fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func @use(%v: !sec.place<!p, "rw">) {
    // expected-error@+1 {{skips 'field#9', which is not a subobject of the destroyed type}}
    "sec.own.destroy_place"(%v) <{destruction_plan = "p", cause = "function-exit", skipped = ["field#9"]}> : (!sec.place<!p, "rw">) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    // expected-error@+1 {{frees the value its custom free is already destroying}}
    func.call @free(%self) : (!sec.ref_mut<!h>) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!p = !sec.struct<identity = "main::P", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "l", type = !h, tags = []>, #sec.struct_field<ordinal = 1, name = "n", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 2, name = "r", type = !h, tags = []>]>
!o = !sec.union<identity = "core::Option", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Some", kind = "single", payload = !h, fields = []>]>
!f = !sec.union<identity = "main::F", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "A", kind = "fields", fields = [#sec.union_field<name = "h", type = !h>]>]>
!r = !sec.result<!h, !sec.int>
!a = !sec.array<!h, "1000000">
!z = !sec.array<!h, "0">
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "p", type = !p, kind = "struct", custom_free = true, free_type = "P", free_function = @free, fields = [{field = 2 : i32, plan = "h"}, {field = 0 : i32, plan = "h"}]}, {id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @hfree}]} {
  func.func @hfree(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func @free(%self: !sec.ref_mut<!p> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    %place = "sec.place.deref"(%self) : (!sec.ref_mut<!p>) -> !sec.place<!p, "rw">
    %field = "sec.place.field"(%place) <{field = 0 : i32}> : (!sec.place<!p, "rw">) -> !sec.place<!h, "rw">
    // expected-error@+1 {{ends a part of the value its custom free is destroying}}
    "sec.own.destroy_place"(%field) <{destruction_plan = "h", cause = "function-exit"}> : (!sec.place<!h, "rw">) -> ()
    return
  }
}

// -----

!r = !sec.result<!sec.int, !sec.int>
// expected-error@+1 {{has custom free on a Result}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "r", type = !r, kind = "active-variant", custom_free = true, free_type = "R", free_function = @free}]} {
  func.func @free(%self: !sec.ref_mut<!r> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
}

// -----

!u = !sec.union<identity = "main::U", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Full", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "Empty", kind = "empty", fields = []>]>
// expected-error@+1 {{has custom free without its source type and function}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "u", type = !u, kind = "active-variant", custom_free = true}]} {
}

// -----

!u = !sec.union<identity = "main::U", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Full", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "Empty", kind = "empty", fields = []>]>
// expected-error@+1 {{has no custom free and no variant steps, so it is trivial}}
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "u", type = !u, kind = "active-variant"}]} {
}
