// RUN: not sec-mlir-opt --sec-verify-union-guards %s -o /dev/null 2>&1 | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 7 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "invalid-union-guards",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @unguarded(%value: !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> !sec.int {
    %payload = "sec.union.unwrap_payload"(%value) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> !sec.int
    return %payload : !sec.int
  }
}

// CHECK: error: 'sec.union.unwrap_payload' op must be in the true successor of a matching union.is_variant guard on the same SSA value
