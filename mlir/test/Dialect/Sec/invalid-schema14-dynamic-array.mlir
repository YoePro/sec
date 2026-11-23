// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// SEC-MLIR-P18 sections 81-96: local verifier rejections of schema-14
// dynamic-array and storage-transition operations.

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @old() {
    // expected-error@+1 {{dynamic-array and storage-transition operations require schema 14}}
    %e = "sec.dynamic_array.empty"() : () -> !sec.int
    return
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @copy(%a: !sec.dynamic_array<!sec.int>) -> !sec.dynamic_array<!sec.int> {
    // expected-error@+1 {{an owning dynamic array is move-only and never copied}}
    %b = "sec.own.copy"(%a) : (!sec.dynamic_array<!sec.int>) -> !sec.dynamic_array<!sec.int>
    return %b : !sec.dynamic_array<!sec.int>
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @readonly(%x: !sec.int, %a: !sec.dynamic_array<!sec.int>) {
    %p = "sec.place.value"(%a) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.dynamic_array<!sec.int>) -> !sec.place<!sec.dynamic_array<!sec.int>, "ro">
    // expected-error@+1 {{changes the owner and requires a writable Place}}
    "sec.dynamic_array.append_prepared"(%p, %x) : (!sec.place<!sec.dynamic_array<!sec.int>, "ro">, !sec.int) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @count() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    // expected-error@+1 {{requires a positive canonical element count}}
    %t = "sec.dynamic_array.has_capacity"(%p) <{count = "0"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    return
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @reclaim() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    // expected-error@+1 {{consumes a resolved owning-value reclamation plan}}
    "sec.storage.reclaim"(%p) <{domain = 1 : i32, authority = "Arena"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    return
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @facts() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    // expected-error@+1 {{pairs a reclamation plan only with owning-value authority}}
    %f, %r = "sec.dynamic_array.grow_backing"(%p) <{count = "1", transition = "advance-epoch", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "Arena", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> (i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    return
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @element(%i: !sec.int) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    // expected-error@+1 {{requires its runtime bounds guard or a branch/analysis length proof}}
    %e = "sec.dynamic_array.element_place"(%p, %i) <{index_signed = true, bounds_kind = "proven", bounds_proof = "constant", backing_domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> !sec.place<!sec.int, "rw">
    return
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @element_constant_proof(%i: !sec.int) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    // expected-error@+1 {{requires its runtime bounds guard or a branch/analysis length proof}}
    %e = "sec.dynamic_array.element_place"(%p, %i) <{index_signed = true, bounds_kind = "proven-safe", bounds_proof = "constant", backing_domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> !sec.place<!sec.int, "rw">
    return
  }
}

// -----

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @semantic(%a: !sec.dynamic_array<!sec.int>) -> !sec.dynamic_array<!sec.int> {
    // expected-error@+1 {{an owning dynamic array is move-only and has no semantic copy}}
    %b = "sec.own.semantic_copy"(%a) <{semantic_copy_plan = "copy"}> : (!sec.dynamic_array<!sec.int>) -> !sec.dynamic_array<!sec.int>
    return %b : !sec.dynamic_array<!sec.int>
  }
}
