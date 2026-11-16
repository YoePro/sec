// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// Dialect v5 §§ 6-7: Result[void, E] uses the void-success convention, `none`
// as its success type. Its Ok has no payload and cannot be unwrapped; the
// error type is never void.

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "property", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @ok() -> !sec.result<none, !sec.core_error<"core::ArithmeticError">> {
    %r = "sec.result.ok"() : () -> !sec.result<none, !sec.core_error<"core::ArithmeticError">>
    %b = "sec.result.is_err"(%r) : (!sec.result<none, !sec.core_error<"core::ArithmeticError">>) -> i1
    return %r : !sec.result<none, !sec.core_error<"core::ArithmeticError">>
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "property", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @payload(%x: !sec.int) {
    // expected-error@+1 {{Ok of a void-success Result takes no payload}}
    %r = "sec.result.ok"(%x) : (!sec.int) -> !sec.result<none, !sec.core_error<"core::ArithmeticError">>
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "property", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unwrap(%r: !sec.result<none, !sec.core_error<"core::ArithmeticError">>) {
    // expected-error@+1 {{a void-success Result has no Ok payload to unwrap}}
    %v = "sec.result.unwrap_ok"(%r) : (!sec.result<none, !sec.core_error<"core::ArithmeticError">>) -> none
    return
  }
}
