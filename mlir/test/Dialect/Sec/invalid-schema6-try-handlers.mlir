// RUN: not sec-mlir-opt --sec-verify-try-handlers %s -o /dev/null 2>&1 | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 6 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "invalid-handlers",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @duplicate(%condition: i1) -> si32 {
    cf.cond_br %condition, ^first, ^second
  ^first:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    cf.br ^merge(%zero : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-variant", sec.try_handler_variant = "Overflow"}
  ^second:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 1 : i32, sec.try_handler_kind = "err-variant", sec.try_handler_variant = "Overflow"}
  ^merge(%value: si32):
    return %value : si32
  }
}

// CHECK: error: 'cf.br' op duplicate or missing Err variant provenance
