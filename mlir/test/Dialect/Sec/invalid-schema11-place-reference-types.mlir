// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module {
  // expected-error@+1 {{sec.place authority must be "ro" or "rw"}}
  func.func private @authority(%value: !sec.place<i32, "wo">)
}

// -----

module {
  // expected-error@+1 {{sec.ref referent must be an ordinary value type, not storage, a Place or a reference}}
  func.func private @nested(%value: !sec.ref<!sec.ref<i32>>)
}

// -----

module {
  // expected-error@+1 {{sec.ref_mut referent must be an ordinary value type, not storage, a Place or a reference}}
  func.func private @storage(%value: !sec.ref_mut<!sec.storage<i32>>)
}

// -----

module {
  // expected-error@+1 {{sec.place referent must be an ordinary value type, not storage, a Place or a reference}}
  func.func private @place_of_ref(%value: !sec.place<!sec.ref<i32>, "ro">)
}

// -----

module {
  // expected-error@+1 {{sec.ref referent type must be non-void}}
  func.func private @void(%value: !sec.ref<none>)
}

// -----

// expected-error@+1 {{Places, direct references and their operations require schema 11, module declares schema 10}}
module attributes {
  sec.dialect_version = 10 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "too-old",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func private @shared(%value: !sec.ref<i32>)
}

// -----

// expected-error@+1 {{Places, direct references and their operations require schema 11, module declares schema 10}}
module attributes {
  sec.dialect_version = 10 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "too-old-op",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @stale() {
    "sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()
  }
}

// -----

// P15 types cannot hide inside aggregate function signatures.
// expected-error@+1 {{Places, direct references and their operations require schema 11, module declares schema 10}}
module attributes {sec.dialect_version = 10 : i32} {
  func.func private @nested(!sec.array<!sec.ref<si32>, "2">)
}

// -----

// Version 1's short metadata path must still reject nested P15 types.
// expected-error@+1 {{Places, direct references and their operations require schema 11, module declares schema 1}}
module attributes {sec.dialect_version = 1 : i32, sec.test_metadata = {nested = [!sec.named<"device::View", !sec.ref_mut<si32>>]}} {}

// -----

// Places are compiler-internal types even when nested in a type attribute.
// expected-error@+1 {{Places, direct references and their operations require schema 11, module declares schema 10}}
module attributes {sec.dialect_version = 10 : i32, sec.test_metadata = {nested = [!sec.place<si32, "ro">]}} {}
