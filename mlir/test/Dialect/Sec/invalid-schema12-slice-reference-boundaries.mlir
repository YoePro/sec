// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

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
  func.func @zero_borrow(%view: !sec.slice<i32>) {
    // expected-error@+1 {{borrow_id and lifetime_id must be non-zero}}
    "sec.ref.end_borrow"(%view) <{borrow_id = 0 : i32, lifetime_id = 2 : i32}> : (!sec.slice<i32>) -> ()
    return
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
  func.func @zero_lifetime(%view: !sec.slice_mut<i32>) {
    // expected-error@+1 {{borrow_id and lifetime_id must be non-zero}}
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 0 : i32}> : (!sec.slice_mut<i32>) -> ()
    return
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
  func.func @validity_array(%view: !sec.array<i32, "3">) {
    // expected-error@+1 {{operand must be !sec.ref or !sec.ref_mut}}
    %valid = "sec.ref.is_valid"(%view) : (!sec.array<i32, "3">) -> i1
    return
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
  func.func @end_array(%view: !sec.array<i32, "3">) {
    // expected-error@+1 {{operand must be !sec.ref or !sec.ref_mut}}
    "sec.ref.end_borrow"(%view) <{borrow_id = 1 : i32, lifetime_id = 2 : i32}> : (!sec.array<i32, "3">) -> ()
    return
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
  func.func @slice_comparison(%view: !sec.slice<i32>) {
    // expected-error@+1 {{compares direct references to the same referent type}}
    %equal = "sec.ref.compare"(%view, %view) <{predicate = "eq"}> : (!sec.slice<i32>, !sec.slice<i32>) -> i1
    return
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
  func.func @element_deref(%view: !sec.slice_mut<i32>) {
    // expected-error@+1 {{operand must be !sec.ref or !sec.ref_mut}}
    %place = "sec.place.deref"(%view) : (!sec.slice_mut<i32>) -> !sec.place<i32, "rw">
    return
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
  // expected-error@+1 {{a dynamic-epoch sec.reference requires invalidation_domain and epoch_dependency}}
  func.func @missing_epoch(%view: !sec.slice<i32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 2 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 3 : i32, address_stability = "Stable", provenance = "arena"}}) {
    return
  }
}
