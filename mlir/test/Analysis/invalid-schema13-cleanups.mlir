// RUN: sec-mlir-opt %s -split-input-file --sec-verify-cleanups -verify-diagnostics

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @missing_exit(%v: !sec.int) {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    // expected-error@+1 {{no sec.cleanup.run_function}}
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @missing_defer_exit() {
    "sec.cleanup.defer_register"() <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 1 : i32}> ({
      "sec.cleanup.defer_yield"() : () -> ()
    }) : () -> ()
    // expected-error@+1 {{no sec.cleanup.run_function}}
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @uncancelled(%v: !sec.int) -> !sec.int {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    %m = "sec.own.move"(%v) : (!sec.int) -> !sec.int
    // expected-error@+1 {{would destroy a moved value}}
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return %m : !sec.int
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @twice(%v: !sec.int) {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    // expected-error@+1 {{not active on every path}}
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @never() {
    // expected-error@+1 {{never registered}}
    "sec.cleanup.cancel"() <{cleanup_action_id = 9 : i32, reason = "moved"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @defer_cancel() {
    "sec.cleanup.defer_register"() <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 1 : i32}> ({
      "sec.cleanup.defer_yield"() : () -> ()
    }) : () -> ()
    // expected-error@+1 {{defer registration}}
    "sec.cleanup.cancel"() <{cleanup_action_id = 1 : i32, reason = "moved"}> : () -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @returned(%v: !sec.int) -> !sec.int {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    // expected-error@+1 {{destroy the returned value}}
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return %v : !sec.int
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @order(%v: !sec.int, %w: !sec.int) {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    // expected-error@+1 {{out of source registration order}}
    "sec.cleanup.track_owned"(%w) <{cleanup_action_id = 2 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @reuse(%v: !sec.int, %w: !sec.int) {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    // expected-error@+1 {{reuses a cleanup action}}
    "sec.cleanup.track_owned"(%w) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @branch_uncancelled(%v: !sec.int, %c: i1) {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    cf.cond_br %c, ^bb1, ^bb2
  ^bb1:
    %m = "sec.own.move"(%v) : (!sec.int) -> !sec.int
    cf.br ^bb2
  ^bb2:
    // expected-error@+1 {{would destroy a moved value}}
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @in_body() {
    "sec.cleanup.defer_register"() <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 1 : i32}> ({
    ^bb0:
      // expected-error@+1 {{not valid inside a defer body}}
      "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
      "sec.cleanup.defer_yield"() : () -> ()
    }) : () -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @not_before_return(%v: !sec.int) {
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    // expected-error@+1 {{immediately precede}}
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    %x = "sec.own.copy"(%v) : (!sec.int) -> !sec.int
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-ops-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @live_on_track(%v: !sec.int, %c: i1) {
    cf.br ^bb1
  ^bb1:
    // expected-error@+1 {{still active on a path}}
    "sec.cleanup.track_owned"(%v) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.int) -> ()
    cf.cond_br %c, ^bb1, ^bb2
  ^bb2:
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @defer_capture(%v: !sec.int, %s: !sec.storage<!sec.int>) {
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
    "sec.cleanup.defer_register"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 1 : i32}> ({
    ^bb0(%c: !sec.place<!sec.int, "rw">):
      "sec.cleanup.defer_yield"() : () -> ()
    }) : (!sec.place<!sec.int, "rw">) -> ()
    %q = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
    // expected-error@+1 {{consumes a value an active defer captures}}
    %m = "sec.own.move_from_place"(%q) : (!sec.place<!sec.int, "rw">) -> !sec.int
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @defer_reads_newer_owner(%v: !sec.int, %s: !sec.storage<!sec.int>) {
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
    "sec.cleanup.defer_register"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 1 : i32}> ({
    ^bb0(%c: !sec.place<!sec.int, "rw">):
      "sec.cleanup.defer_yield"() : () -> ()
    }) : (!sec.place<!sec.int, "rw">) -> ()
    %q = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
    // expected-error@+1 {{registers an owner an active defer reads, whose cleanup would run before the defer}}
    "sec.cleanup.track_owned"(%q) <{cleanup_action_id = 2 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, destruction_plan = "p"}> : (!sec.place<!sec.int, "rw">) -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @defer_order(%s: !sec.storage<!sec.int>) {
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
    "sec.cleanup.defer_register"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 2 : i32, defer_id = 1 : i32}> ({
    ^bb0(%c: !sec.place<!sec.int, "rw">):
      "sec.cleanup.defer_yield"() : () -> ()
    }) : (!sec.place<!sec.int, "rw">) -> ()
    // expected-error@+1 {{registers out of source registration order}}
    "sec.cleanup.defer_register"(%p) <{cleanup_action_id = 2 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, defer_id = 2 : i32}> ({
    ^bb0(%c: !sec.place<!sec.int, "rw">):
      "sec.cleanup.defer_yield"() : () -> ()
    }) : (!sec.place<!sec.int, "rw">) -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "own", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @defer_reads_inner_owner(%s: !sec.storage<!sec.int>) {
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.int>) -> !sec.place<!sec.int, "rw">
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 2 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "p"}> : (!sec.place<!sec.int, "rw">) -> ()
    "sec.cleanup.defer_register"(%p) <{cleanup_action_id = 2 : i32, cleanup_scope_id = 2 : i32, cleanup_registration_ordinal = 2 : i32, defer_id = 1 : i32}> ({
    ^bb0(%c: !sec.place<!sec.int, "rw">):
      "sec.cleanup.defer_yield"() : () -> ()
    }) : (!sec.place<!sec.int, "rw">) -> ()
    // expected-error@+1 {{would destroy an owner an active defer reads before the defer runs}}
    "sec.cleanup.run_scope"() <{scope_ids = array<i32: 2>, exit_kind = "fallthrough"}> : () -> ()
    "sec.cleanup.run_function"() <{exit_kind = "return"}> : () -> ()
    return
  }
}
