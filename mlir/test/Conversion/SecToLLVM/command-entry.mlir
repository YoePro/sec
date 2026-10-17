// RUN: sec-mlir-opt %s --split-input-file --sec-lower-command-entry --verify-diagnostics | FileCheck %s

// CHECK-LABEL: func.func @main() -> i32
// CHECK: %[[S:.*]] = call @sec_fn_0() : () -> i64
// CHECK: %[[T:.*]] = arith.trunci %[[S]] : i64 to i32
// CHECK: return %[[T]] : i32
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0() -> i64 attributes {sec.function_id = "main::main()", sec.source_name = "main", sec.extern = false, sec.unsafe = false} {
    %c = arith.constant 0 : i64
    return %c : i64
  }
}

// -----

// expected-error @+1 {{command entry 'main::main()' is not defined in this module}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0() -> i64 attributes {sec.function_id = "main::Other()", sec.source_name = "main", sec.extern = false, sec.unsafe = false} {
    %c = arith.constant 0 : i64
    return %c : i64
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // expected-error @+1 {{supports only the command entry `fn main() int`}}
  func.func @sec_fn_0(%x: i64) -> i64 attributes {sec.function_id = "main::main()", sec.source_name = "main", sec.extern = false, sec.unsafe = false} {
    return %x : i64
  }
}
