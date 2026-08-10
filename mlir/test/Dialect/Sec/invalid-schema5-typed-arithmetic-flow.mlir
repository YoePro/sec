// RUN: not sec-mlir-opt --split-input-file %s 2>&1 | FileCheck %s

// CHECK: op invalid arithmetic failure reason
module attributes {sec.dialect_version = 5 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "bad", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x", sec.target_abi = "x", sec.target_profile = "x", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad_reason() {
    %reason = "sec.arithmetic_failure_reason.constant"() <{value = "other"}> : () -> !sec.arithmetic_failure_reason
    return
  }
}

// -----

// CHECK: op payload type must exactly match Result component
module attributes {sec.dialect_version = 5 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "bad", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x", sec.target_abi = "x", sec.target_profile = "x", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad_ok(%value: ui32) {
    %result = "sec.result.ok"(%value) : (ui32) -> !sec.result<si32, !sec.core_error<"core::ArithmeticError">>
    return
  }
}

// -----

// CHECK: op cannot consume the none arithmetic failure reason
module attributes {sec.dialect_version = 5 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "bad", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x", sec.target_abi = "x", sec.target_profile = "x", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad_failure() {
    %reason = "sec.arithmetic_failure_reason.constant"() <{value = "none"}> : () -> !sec.arithmetic_failure_reason
    "sec.fail.arithmetic"(%reason) {sec.operator = "+"} : (!sec.arithmetic_failure_reason) -> ()
  }
}

