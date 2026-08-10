// RUN: sec-mlir-opt %s | FileCheck %s
// RUN: sec-mlir-opt --sec-verify-checked-integer-guards %s -o /dev/null

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 5 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "typed-arithmetic",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @fallible(%left: si128, %right: si128) -> !sec.result<si128, !sec.core_error<"core::ArithmeticError">> {
    %result, %failed, %reason = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si128, si128) -> (si128, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %failed, ^failure(%reason : !sec.arithmetic_failure_reason), ^success
  ^failure(%why: !sec.arithmetic_failure_reason):
    %error = "sec.arithmetic_error.from_reason"(%why) : (!sec.arithmetic_failure_reason) -> !sec.core_error<"core::ArithmeticError">
    %failed_result = "sec.result.err"(%error) : (!sec.core_error<"core::ArithmeticError">) -> !sec.result<si128, !sec.core_error<"core::ArithmeticError">>
    return %failed_result : !sec.result<si128, !sec.core_error<"core::ArithmeticError">>
  ^success:
    %ok = "sec.result.ok"(%result) : (si128) -> !sec.result<si128, !sec.core_error<"core::ArithmeticError">>
    return %ok : !sec.result<si128, !sec.core_error<"core::ArithmeticError">>
  }

  func.func @ordinary(%left: ui256, %right: ui256) -> ui256 {
    %result, %failed, %reason = "sec.int.binary_checked"(%left, %right) <{kind = "multiply"}> : (ui256, ui256) -> (ui256, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %failed, ^failure(%reason : !sec.arithmetic_failure_reason), ^success
  ^failure(%why: !sec.arithmetic_failure_reason):
    "sec.fail.arithmetic"(%why) {sec.operator = "*"} : (!sec.arithmetic_failure_reason) -> ()
  ^success:
    return %result : ui256
  }
}

// CHECK: sec.dialect_version = 5 : i32
// CHECK: !sec.result<si128, !sec.core_error<"core::ArithmeticError">>
// CHECK: "sec.int.binary_checked"
// CHECK-SAME: -> (si128, i1, !sec.arithmetic_failure_reason)
// CHECK: ^bb{{.*}}(%{{.*}}: !sec.arithmetic_failure_reason):
// CHECK: "sec.arithmetic_error.from_reason"
// CHECK: "sec.result.err"
// CHECK: "sec.result.ok"
// CHECK: "sec.fail.arithmetic"(%{{.*}})

