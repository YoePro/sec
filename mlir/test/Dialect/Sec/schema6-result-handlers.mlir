// RUN: sec-mlir-opt %s | FileCheck %s
// RUN: sec-mlir-opt --sec-verify-result-guards --sec-verify-try-handlers %s -o /dev/null

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 6 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "result-handlers",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @handle(%input: !sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> si32 {
    %is_err = "sec.result.is_err"(%input) : (!sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> i1
    cf.cond_br %is_err, ^error, ^success
  ^error:
    %failure = "sec.result.unwrap_err"(%input) : (!sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> !sec.core_error<"core::ArithmeticError">
    %is_division = "sec.core_error.is_variant"(%failure) <{variant = "DivisionByZero"}> {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-variant", sec.try_handler_variant = "DivisionByZero"} : (!sec.core_error<"core::ArithmeticError">) -> i1
    cf.cond_br %is_division, ^division, ^catch_all
  ^success:
    %value = "sec.result.unwrap_ok"(%input) : (!sec.result<si32, !sec.core_error<"core::ArithmeticError">>) -> si32
    cf.br ^merge(%value : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^division:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    cf.br ^merge(%zero : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-variant", sec.try_handler_variant = "DivisionByZero"}
  ^catch_all:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 1 : i32, sec.try_handler_kind = "err-catch-all"}
  ^merge(%resolved: si32):
    return %resolved : si32
  }
}

// CHECK: sec.dialect_version = 6 : i32
// CHECK: "sec.result.is_err"
// CHECK: "sec.result.unwrap_err"
// CHECK: "sec.core_error.is_variant"
// CHECK: "sec.result.unwrap_ok"
// CHECK: sec.try_handler_kind = "err-catch-all"
