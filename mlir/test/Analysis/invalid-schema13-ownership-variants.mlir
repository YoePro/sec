// RUN: sec-mlir-opt %s -split-input-file --sec-verify-ownership -verify-diagnostics
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 14, 25-26, 63.

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @replace_needs_if_owned(%c: i1) {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = []} : () -> !o
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "o", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!o>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!o>) -> !sec.place<!o, "rw">
    "sec.own.initialize_place"(%p, %o) : (!sec.place<!o, "rw">, !o) -> ()
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "o"}> : (!sec.place<!o, "rw">) -> ()
    cf.cond_br %c, ^moved, ^join
    ^moved:
    %m = "sec.own.move_from_place"(%p) : (!sec.place<!o, "rw">) -> !o
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    "sec.call.direct"(%m) <{callee = @takeOption}> {sec.argument_actions = ["move"]} : (!o) -> ()
    cf.br ^join
    ^join:
    %n = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = []} : () -> !o
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    "sec.own.replace_place"(%p, %n) <{destruction_plan = "o", old_cleanup_action_id = 1 : i32, new_cleanup_action_id = 2 : i32}> : (!sec.place<!o, "rw">, !o) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @use_after_conditional_discard(%c: i1, %v: !h) {
    cf.cond_br %c, ^moved, ^join
    ^moved:
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    cf.br ^join
    ^join:
    "sec.own.discard"(%v) <{destruction_plan = "h", discard_origin = "explicit", if_owned}> : (!h) -> ()
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @candidate_cannot_be_taken(%o: !o) {
    %p = "sec.place.value"(%o) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!o) -> !sec.place<!o, "ro">
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^matched, ^rest
    ^matched:
    %q = "sec.place.union_payload"(%p) <{variant = 0 : i32, variant_proven = true}> : (!sec.place<!o, "ro">) -> !sec.place<!h, "ro">
    // expected-error@+1 {{a guard candidate may only be inspected}}
    %v = "sec.place.read"(%q) <{action = "candidate"}> : (!sec.place<!h, "ro">) -> !h
    "sec.call.direct"(%v) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^rest:
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @candidate_dies_with_its_payload() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = []} : () -> !o
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "o", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!o>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!o>) -> !sec.place<!o, "rw">
    "sec.own.initialize_place"(%p, %o) : (!sec.place<!o, "rw">, !o) -> ()
    %r = "sec.place.read"(%p) <{action = "observe"}> : (!sec.place<!o, "rw">) -> !o
    %c = "sec.union.is_variant"(%r) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^matched, ^rest
    ^matched:
    %q = "sec.place.union_payload"(%p) <{variant = 0 : i32, variant_proven = true}> : (!sec.place<!o, "rw">) -> !sec.place<!h, "rw">
    %v = "sec.place.read"(%q) <{action = "candidate"}> : (!sec.place<!h, "rw">) -> !h
    %h = "sec.own.move_from_place"(%q) : (!sec.place<!h, "rw">) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    %n = "sec.struct.extract"(%v) <{action = "copy-trivial", field = 0 : i32}> : (!h) -> !sec.int
    return
    ^rest:
    %m = "sec.own.move_from_place"(%p) : (!sec.place<!o, "rw">) -> !o
    "sec.own.destroy_value"(%m) <{destruction_plan = "o", cause = "temporary-end"}> : (!o) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @observe_after_move() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = []} : () -> !o
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "o", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!o>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!o>) -> !sec.place<!o, "rw">
    "sec.own.initialize_place"(%p, %o) : (!sec.place<!o, "rw">, !o) -> ()
    %m = "sec.own.move_from_place"(%p) : (!sec.place<!o, "rw">) -> !o
    "sec.own.destroy_value"(%m) <{destruction_plan = "o", cause = "temporary-end"}> : (!o) -> ()
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    %v = "sec.place.read"(%p) <{action = "observe"}> : (!sec.place<!o, "rw">) -> !o
    %c = "sec.union.is_variant"(%v) <{variant = 0 : i32}> : (!o) -> i1
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @observation_is_not_a_value(%p: !sec.ref_mut<!o>) {
    %q = "sec.place.deref"(%p) : (!sec.ref_mut<!o>) -> !sec.place<!o, "rw">
    // expected-error@+1 {{an observing read may only feed an active-variant test}}
    %v = "sec.place.read"(%q) <{action = "observe"}> : (!sec.place<!o, "rw">) -> !o
    "sec.call.direct"(%v) <{callee = @takeOption}> {sec.argument_actions = ["move"]} : (!o) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @registered() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = []} : () -> !o
    "sec.cleanup.track_owned"(%o) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "o"}> : (!o) -> ()
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^none
    ^some:
    // expected-error@+1 {{transfers a value still registered for cleanup without sec.own.move}}
    %h = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^none:
    "sec.cleanup.run_function"() <{exit_kind = "fallthrough"}> : () -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @twice() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = []} : () -> !o
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^none
    ^some:
    %h = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    // expected-error@+1 {{uses a value that is moved, destroyed or uninitialized on some path}}
    %g = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%g) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^none:
    "sec.own.destroy_value"(%o) <{destruction_plan = "o", cause = "temporary-end"}> : (!o) -> ()
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @none_arm_leaks() {
    %o = "sec.call.direct"() <{callee = @makeOption}> {sec.argument_actions = []} : () -> !o
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^none
    ^some:
    %h = "sec.union.unwrap_payload"(%o) <{payloadAction = "move", variant = 0 : i32}> : (!o) -> !h
    "sec.call.direct"(%h) <{callee = @sink}> {sec.argument_actions = ["move"]} : (!h) -> ()
    return
    ^none:
    // expected-error@+1 {{leaves an owned value with no terminal ownership action}}
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @unknown_projection_action(%o: !o) {
    %c = "sec.union.is_variant"(%o) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %c, ^some, ^none
    ^some:
    // expected-error@+1 {{Package 11 union payload action must be copy-trivial}}
    %h = "sec.union.unwrap_payload"(%o) <{payloadAction = "construct-direct", variant = 0 : i32}> : (!o) -> !h
    return
    ^none:
    return
  }
}

// -----

!h = !sec.struct<identity = "main::H", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "id", type = !sec.int, tags = []>]>
!f = !sec.result<!sec.int, !h>
!o = !sec.union<identity = "core::Option<main::H>", typeArguments = [!h], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !h, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = [{id = "h", type = !h, kind = "struct", custom_free = true, free_type = "H", free_function = @free}, {id = "f", type = !f, kind = "active-variant", variants = [{variant = 1 : i32, plan = "h"}]}, {id = "o", type = !o, kind = "active-variant", variants = [{variant = 0 : i32, plan = "h"}]}]} {
  func.func @free(%self: !sec.ref_mut<!h> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "receiver"}}) {
    return
  }
  func.func private @make() -> !h attributes {sec.function_id = "make", sec.source_name = "make", sec.extern = false, sec.unsafe = false}
  func.func private @sink(!h) attributes {sec.function_id = "sink", sec.source_name = "sink", sec.extern = false, sec.unsafe = false}
  func.func private @makeOption() -> !o attributes {sec.function_id = "makeOption", sec.source_name = "makeOption", sec.extern = false, sec.unsafe = false}
  func.func private @takeOption(!o) attributes {sec.function_id = "takeOption", sec.source_name = "takeOption", sec.extern = false, sec.unsafe = false}
  func.func private @makeFailure() -> !f attributes {sec.function_id = "makeFailure", sec.source_name = "makeFailure", sec.extern = false, sec.unsafe = false}
  func.func @unknown_construct_action() -> !o {
    %h = "sec.call.direct"() <{callee = @make}> {sec.argument_actions = []} : () -> !h
    // expected-error@+1 {{Package 11 union payload action must be copy-trivial}}
    %o = "sec.union.construct"(%h) <{fieldNames = [], payloadActions = ["move"], variant = 0 : i32}> : (!h) -> !o
    return %o : !o
  }
}
