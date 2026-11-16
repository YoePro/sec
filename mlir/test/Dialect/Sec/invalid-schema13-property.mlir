// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// Semantic IR § 26: a property accessor call keeps its property operation in
// sec.property; a valid one passes, a malformed one or one on another
// operation is rejected.

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "property", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func private @get() -> !sec.int attributes {sec.function_id = "get", sec.source_name = "Count.get", sec.extern = false, sec.unsafe = false}
  func.func @use() -> !sec.int {
    %v = "sec.call.direct"() <{callee = @get}> {sec.argument_actions = [], sec.property = {kind = "read", name = "Count", owner = "main::Counter", static = true}} : () -> !sec.int
    return %v : !sec.int
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "property", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func private @get() -> !sec.int attributes {sec.function_id = "get", sec.source_name = "Count.get", sec.extern = false, sec.unsafe = false}
  func.func @use() -> !sec.int {
    // expected-error@+1 {{sec.property has unknown kind "peek"}}
    %v = "sec.call.direct"() <{callee = @get}> {sec.argument_actions = [], sec.property = {kind = "peek", name = "Count", owner = "main::Counter", static = true}} : () -> !sec.int
    return %v : !sec.int
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "property", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func private @get() -> !sec.int attributes {sec.function_id = "get", sec.source_name = "Count.get", sec.extern = false, sec.unsafe = false}
  func.func @use() -> !sec.int {
    // expected-error@+1 {{sec.property requires exactly kind, name, owner and static}}
    %v = "sec.call.direct"() <{callee = @get}> {sec.argument_actions = [], sec.property = {kind = "read", name = "Count"}} : () -> !sec.int
    return %v : !sec.int
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "property", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func private @get() -> !sec.int attributes {sec.function_id = "get", sec.source_name = "Count.get", sec.extern = false, sec.unsafe = false}
  func.func @use() -> !sec.int {
    // expected-error@+1 {{sec.property is a dictionary on sec.call.direct}}
    %v = "sec.const.int"() <{value = "1"}> {sec.property = {kind = "read", name = "Count", owner = "main::Counter", static = true}} : () -> !sec.int
    return %v : !sec.int
  }
}
