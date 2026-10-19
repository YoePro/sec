// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-reference-boundaries",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @bad() {
    // expected-error@+1 {{slice bounds provenance requires schema 12}}
    "sec.fail.bounds"() {operation = "slice-index"} : () -> ()
  }
}

// -----

module attributes {
  sec.dialect_version = 10 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-reference-boundaries",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @bad() {
    // expected-error@+1 {{slice bounds provenance requires schema 12}}
    "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-reference-boundaries",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @bad() {
    // expected-error@+1 {{operation must be fixed-array-index}}
    "sec.fail.bounds"() {operation = "slice-length"} : () -> ()
  }
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-reference-boundaries",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @bad() {
    // expected-error@+1 {{operation must be fixed-array-index}}
    "sec.fail.bounds"() : () -> ()
  }
}

// -----

module {
  func.func @undeclared(%view: !sec.slice<i32>) {
    // expected-error@+1 {{slice reference operations require schema 12}}
    %valid = "sec.ref.is_valid"(%view) : (!sec.slice<i32>) -> i1
    return
  }
}

// -----

module {
  func.func @undeclared_end(%view: !sec.slice_mut<i32>) {
    // expected-error@+1 {{slice reference operations require schema 12}}
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.slice_mut<i32>) -> ()
    return
  }
}
