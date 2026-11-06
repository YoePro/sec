// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @old_schema(%v: !sec.int) {
    // expected-error@+1 {{require schema 13}}
    %a = "sec.own.move"(%v) : (!sec.int) -> !sec.int
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @copy_mutable(%r: !sec.ref_mut<!sec.int>) {
    // expected-error@+1 {{mutable reference is never copied}}
    %a = "sec.own.copy"(%r) : (!sec.ref_mut<!sec.int>) -> !sec.ref_mut<!sec.int>
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @copy_type(%v: !sec.int, %w: !sec.uint) {
    // expected-error@+1 {{preserves the value type}}
    %a = "sec.own.move"(%v) : (!sec.int) -> !sec.uint
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @read_only_move(%p: !sec.place<!sec.int, "ro">) {
    // expected-error@+1 {{writable Place}}
    %a = "sec.own.move_from_place"(%p) : (!sec.place<!sec.int, "ro">) -> !sec.int
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @mode(%p: !sec.place<!sec.int, "rw">) {
    // expected-error@+1 {{invalid copy mode}}
    %a = "sec.own.copy_from_place"(%p) <{copy_mode = "bits"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  // MD-125: equal identities inherit the live registration; a conditional
  // replacement must also name a distinct new one.
  func.func @same_ids(%p: !sec.place<!sec.int, "rw">, %v: !sec.int) {
    // expected-error@+1 {{when conditional, a distinct new one}}
    "sec.own.replace_place"(%p, %v) <{destruction_plan = "p", old_cleanup_action_id = 1 : i32, new_cleanup_action_id = 1 : i32, if_owned}> : (!sec.place<!sec.int, "rw">, !sec.int) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @discard_ref(%r: !sec.ref<!sec.int>) {
    // expected-error@+1 {{never destroys its referent}}
    "sec.own.discard"(%r) <{discard_origin = "explicit", destruction_plan = "p"}> : (!sec.ref<!sec.int>) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @destroy_ref(%r: !sec.ref<!sec.int>) {
    // expected-error@+1 {{ended, never destroyed}}
    "sec.own.destroy_value"(%r) <{destruction_plan = "p", cause = "cleanup"}> : (!sec.ref<!sec.int>) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @track_ref(%r: !sec.ref<!sec.int>) {
    // expected-error@+1 {{borrow holder is not owned}}
    "sec.cleanup.track_owned"(%r) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.ref<!sec.int>) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @cancel() {
    // expected-error@+1 {{valid reason}}
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "because"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @defer_args(%p: !sec.place<!sec.int, "rw">) {
    // expected-error@+1 {{match the capture types}}
    "sec.cleanup.defer_register"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 1 : i32}> ({
    ^bb0:
      "sec.cleanup.defer_yield"() : () -> ()
    }) : (!sec.place<!sec.int, "rw">) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @defer_outer(%v: !sec.int) {
    // expected-error@+1 {{nor uses an outer value}}
    "sec.cleanup.defer_register"() <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 1 : i32}> ({
      %a = "sec.own.copy"(%v) : (!sec.int) -> !sec.int
      "sec.cleanup.defer_yield"() : () -> ()
    }) : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @yield_outside() {
    // expected-error@+1 {{must be inside a defer body}}
    "sec.cleanup.defer_yield"() : () -> ()
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @function_exit() {
    // expected-error@+1 {{invalid exit kind}}
    "sec.cleanup.run_function"() <{exit_kind = "break"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @panic_exit() {
    // expected-error@+1 {{invalid exit kind}}
    "sec.cleanup.run_scope"() <{scope_ids = array<i32: 1>, exit_kind = "panic"}> : () -> ()
    return
  }
}

// -----

// MD-125: unless_active names live registrations by non-zero identity.
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "unless-active", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unless_zero(%p: !sec.place<!sec.int, "rw">) {
    // expected-error@+1 {{unless_active must name non-zero registrations}}
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 2 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p", unless_active = array<i32: 0>}> : (!sec.place<!sec.int, "rw">) -> ()
    return
  }
}
