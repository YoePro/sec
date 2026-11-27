// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// Nominal identity survives static verification; only the initializer is
// checked against its selected base representation (semantic_ir.md §§7,28;
// static.md §§15-16). Malformed nominal leaves remain invalid.

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "2", is_mutable = false, sym_name = "value", type = !sec.named<"main::Positive", !sec.int>}> : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "2", is_mutable = false, sym_name = "value", type = !sec.named<"main::Alias", !sec.named<"main::Positive", si32>>}> : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "true", is_mutable = false, sym_name = "value", type = !sec.distinct<"main::Flag", i1>}> : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "bits:1056964608", is_mutable = false, sym_name = "value", type = !sec.named<"main::Ratio", f32>}> : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "aggregate", is_mutable = false, sym_name = "value", type = !sec.array<!sec.named<"main::Positive", si32>, "2">}> {sec.static_initializer = ["2", "3"]} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{integer static requires a decimal initial value}}
  "sec.static.global"() <{initial = "true", is_mutable = false, sym_name = "value", type = !sec.named<"main::Count", !sec.int>}> : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{bool static requires initial value true or false}}
  "sec.static.global"() <{initial = "2", is_mutable = false, sym_name = "value", type = !sec.named<"main::Flag", i1>}> : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{float static requires its canonical bit pattern}}
  "sec.static.global"() <{initial = "bits:4294967296", is_mutable = false, sym_name = "value", type = !sec.named<"main::Ratio", f32>}> : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{bool static component must be true or false}}
  "sec.static.global"() <{initial = "aggregate", is_mutable = false, sym_name = "value", type = !sec.array<!sec.named<"main::Flag", i1>, "1">}> {sec.static_initializer = ["2"]} : () -> ()
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "statics", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error@+1 {{integer static component requires a decimal value}}
  "sec.static.global"() <{initial = "aggregate", is_mutable = false, sym_name = "value", type = !sec.array<!sec.named<"main::Count", si32>, "1">}> {sec.static_initializer = ["false"]} : () -> ()
}
