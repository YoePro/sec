// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-115 §§2,6,10: no repair, truncation or predecessor-schema admission.
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[3..<1]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[-1..<3]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[01..<3]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[1..<03]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[1 ..<3]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[1..<]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[..]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[?"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[*"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#4294967296[?]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{is not a canonical origin}}
  func.func private @bad(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[4294967296]"]}
}

// -----

module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{returned slice range projections require schema 12}}
  func.func private @old(!sec.ref<!sec.array<si32, "4">>) -> !sec.ref<si32> attributes {sec.return_origins = ["param#0[?][*]"]}
}

// -----

module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-return-range",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // expected-error@+1 {{contains duplicate entry}}
  func.func private @duplicate(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0[?]", "param#0[?]"]}
}
