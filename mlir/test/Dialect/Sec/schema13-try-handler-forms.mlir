// RUN: sec-mlir-opt --sec-verify-try-handlers %s | FileCheck %s
// MD-031 handler forms (rules/mlir/sec_mlir_dialect.md v11.41.6): a guarded
// handler neither covers nor ends the list, a partial list propagates through
// its residual func.return, and an Option None handler ends the list.

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect = "sec",
  sec.dialect_version = 13 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema13-try-handler-forms",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  // CHECK-LABEL: func.func @guarded
  func.func @guarded(%failed: i1, %guard: i1) -> si32 {
    cf.cond_br %failed, ^failure, ^success
  ^success:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    cf.br ^merge(%zero : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^failure:
    cf.cond_br %guard, ^first, ^second
  ^first:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-catch-all", sec.try_handler_exhaustive = true, sec.try_handler_guarded = true}
  ^second:
    %two = "sec.const.int"() <{value = "2"}> : () -> si32
    cf.br ^merge(%two : si32) {sec.try_handler_index = 1 : i32, sec.try_handler_kind = "err-catch-all", sec.try_handler_exhaustive = true}
  ^merge(%value: si32):
    return %value : si32
  }
  // CHECK-LABEL: func.func @partial
  func.func @partial(%failed: i1, %matched: i1) -> si32 {
    cf.cond_br %failed, ^failure, ^success
  ^success:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    cf.br ^merge(%zero : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^failure:
    cf.cond_br %matched, ^handled, ^rest
  ^handled:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-variant", sec.try_handler_variant = "NotFound", sec.try_handler_residual = true}
  ^rest:
    %minus = "sec.const.int"() <{value = "-1"}> : () -> si32
    return {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "residual"} %minus : si32
  ^merge(%value: si32):
    return %value : si32
  }
  // CHECK-LABEL: func.func @option_none
  func.func @option_none(%absent: i1) -> si32 {
    cf.cond_br %absent, ^none, ^some
  ^some:
    %four = "sec.const.int"() <{value = "4"}> : () -> si32
    cf.br ^merge(%four : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^none:
    %nine = "sec.const.int"() <{value = "9"}> : () -> si32
    cf.br ^merge(%nine : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "none", sec.try_handler_exhaustive = true}
  ^merge(%value: si32):
    return %value : si32
  }
}
