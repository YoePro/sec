// RUN: not sec-mlir-opt --sec-verify-result-guards %s -o /dev/null 2>&1 | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 6 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "invalid-result-guard",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @wrong_order(%input: !sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> si32 {
    %is_err = "sec.result.is_err"(%input) : (!sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> i1
    cf.cond_br %is_err, ^success, ^error
  ^error:
    %failure = "sec.result.unwrap_err"(%input) : (!sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> !sec.core_error<"core::ArithmeticError">
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    return %zero : si32
  ^success:
    %value = "sec.result.unwrap_ok"(%input) : (!sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> si32
    return %value : si32
  }
}

// CHECK: error: 'sec.result.is_err' op true successor must begin by unwrapping Err from the tested Result
