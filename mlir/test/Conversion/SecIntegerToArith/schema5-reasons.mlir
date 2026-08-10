// RUN: sec-mlir-opt --sec-lower-checked-integers %s | FileCheck %s

module attributes {sec.dialect_version = 5 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "reasons", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x", sec.target_abi = "x", sec.target_profile = "x", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @divide(%left: si32, %right: si32) -> si32 {
    %result, %failed, %reason = "sec.int.binary_checked"(%left, %right) <{kind = "divide"}> : (si32, si32) -> (si32, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %failed, ^failure(%reason : !sec.arithmetic_failure_reason), ^success
  ^failure(%why: !sec.arithmetic_failure_reason):
    "sec.fail.arithmetic"(%why) {sec.operator = "/"} : (!sec.arithmetic_failure_reason) -> ()
  ^success:
    return %result : si32
  }

  func.func @shift(%value: si32, %count: si8) -> si32 {
    %result, %failed, %reason = "sec.int.shift_checked"(%value, %count) <{kind = "left_signed"}> : (si32, si8) -> (si32, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %failed, ^failure(%reason : !sec.arithmetic_failure_reason), ^success
  ^failure(%why: !sec.arithmetic_failure_reason):
    "sec.fail.arithmetic"(%why) {sec.operator = "<<"} : (!sec.arithmetic_failure_reason) -> ()
  ^success:
    return %result : si32
  }
}

// CHECK-LABEL: func.func @divide
// CHECK: value = "overflow"
// CHECK: arith.select
// CHECK: value = "division-by-zero"
// CHECK: arith.select
// CHECK: cf.cond_br
// CHECK-LABEL: func.func @shift
// CHECK: value = "overflow"
// CHECK: arith.select
// CHECK: value = "invalid-shift"
// CHECK: arith.select
// CHECK: cf.cond_br
