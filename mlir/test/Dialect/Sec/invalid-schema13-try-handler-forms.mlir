// RUN: sec-mlir-opt --split-input-file --verify-diagnostics --sec-verify-try-handlers %s
// MD-031 handler forms that must be rejected (rules/mlir/sec_mlir_dialect.md v11.41.6).

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.dialect = "sec", sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-schema13-try-handler-forms", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little"} {
  // expected-error@+1 {{partial try handlers need a residual func.return}}
  func.func @no_residual_return(%failed: i1) -> si32 {
    cf.cond_br %failed, ^failure, ^success
  ^success:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    cf.br ^merge(%zero : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^failure:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-variant", sec.try_handler_variant = "NotFound", sec.try_handler_residual = true}
  ^merge(%value: si32):
    return %value : si32
  }
}

// -----

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.dialect = "sec", sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-schema13-try-handler-forms", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little"} {
  // expected-error@+1 {{Err catch-all must follow all specific handlers for a try merge}}
  func.func @after_catch_all(%failed: i1, %guard: i1) -> si32 {
    cf.cond_br %failed, ^failure, ^success
  ^success:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    cf.br ^merge(%zero : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^failure:
    cf.cond_br %guard, ^first, ^second
  ^first:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 1 : i32, sec.try_handler_kind = "err-catch-all", sec.try_handler_exhaustive = true, sec.try_handler_guarded = true}
  ^second:
    %two = "sec.const.int"() <{value = "2"}> : () -> si32
    cf.br ^merge(%two : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "err-catch-all", sec.try_handler_exhaustive = true}
  ^merge(%value: si32):
    return %value : si32
  }
}

// -----

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.dialect = "sec", sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-schema13-try-handler-forms", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little"} {
  func.func @residual_branch(%failed: i1) -> si32 {
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    // expected-error@+1 {{residual try propagation must be a func.return with index -1}}
    cf.br ^merge(%zero : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "residual"}
  ^merge(%value: si32):
    return %value : si32
  }
}

// -----

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.dialect = "sec", sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "invalid-schema13-try-handler-forms", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little"} {
  func.func @duplicate_none(%absent: i1, %other: i1) -> si32 {
    cf.cond_br %absent, ^none, ^some
  ^some:
    %four = "sec.const.int"() <{value = "4"}> : () -> si32
    cf.br ^merge(%four : si32) {sec.try_handler_index = -1 : i32, sec.try_handler_kind = "ok"}
  ^none:
    cf.cond_br %other, ^first, ^second
  ^first:
    %one = "sec.const.int"() <{value = "1"}> : () -> si32
    cf.br ^merge(%one : si32) {sec.try_handler_index = 0 : i32, sec.try_handler_kind = "none", sec.try_handler_exhaustive = true}
  ^second:
    %two = "sec.const.int"() <{value = "2"}> : () -> si32
    // expected-error@+1 {{try handler follows an unguarded catch-all}}
    cf.br ^merge(%two : si32) {sec.try_handler_index = 1 : i32, sec.try_handler_kind = "none", sec.try_handler_exhaustive = true}
  ^merge(%value: si32):
    return %value : si32
  }
}
