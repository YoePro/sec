// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

// expected-error@+1 {{slice types require schema 12, module declares schema 11}}
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func private @shared(!sec.slice<i32>)
}

// -----

// expected-error@+1 {{slice types require schema 12, module declares schema 10}}
module attributes {
  sec.dialect_version = 10 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func private @mutable(!sec.slice_mut<i32>)
}

// -----

// expected-error@+1 {{slice types require schema 12, module declares schema 11}}
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func private @nested(!sec.named<"Wrapper", !sec.array<!sec.slice_mut<i32>, "4">>)
}

// -----

// expected-error@+1 {{slice types require schema 12, module declares schema 1}}
module attributes {
  sec.dialect_version = 1 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func private @old(!sec.slice<i32>)
}

// -----

// expected-error@+1 {{slice types require schema 12, module declares schema 11}}
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @block_argument() {
    return
  ^unreachable(%value: !sec.slice<i32>):
    return
  }
}

// -----

// expected-error@+1 {{slice types require schema 12, module declares schema 11}}
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @result() {
    %value = "builtin.unrealized_conversion_cast"() : () -> !sec.slice_mut<i32>
    return
  }
}
