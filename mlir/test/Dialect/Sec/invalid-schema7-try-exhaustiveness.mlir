// RUN: not sec-mlir-opt --sec-verify-try-handlers %s -o /dev/null 2>&1 | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect = "sec",
  sec.dialect_version = 7 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "invalid-schema7-try-exhaustiveness",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @missing_exhaustive_provenance(%condition: i1) -> si32 {
    cf.cond_br %condition, ^success, ^failure
  ^success:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    cf.br ^merge(%zero : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^failure:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-variant", sec.try_handler_variant = "Failed"}
  ^merge(%value: si32):
    return %value : si32
  }
}

// CHECK: try handlers require exhaustive Sema provenance
