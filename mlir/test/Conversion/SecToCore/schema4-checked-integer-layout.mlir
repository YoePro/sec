// RUN: sec-mlir-opt --sec-resolve-scalar-layout %s | FileCheck %s
// RUN: sec-mlir-opt --sec-verify-checked-integer-guards --sec-lower-scalar-core --sec-verify-checked-integer-guards %s -o /dev/null

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 4 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "checked-layout",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @add(%left: !sec.int, %right: !sec.int) -> !sec.int {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (!sec.int, !sec.int) -> (!sec.int, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : !sec.int
  }

  func.func @compare(%left: !sec.uint, %right: !sec.uint) -> i1 {
    %result = "sec.int.cmp"(%left, %right) <{predicate = "lt"}> : (!sec.uint, !sec.uint) -> i1
    return %result : i1
  }

  func.func @wide(%signed: si128, %unsigned: ui256) {
    %signed_copy = "sec.int.unary_plus"(%signed) : (si128) -> si128
    %unsigned_copy = "sec.int.unary_plus"(%unsigned) : (ui256) -> ui256
    return
  }
}

// CHECK-LABEL: func.func @add(
// CHECK-SAME: %[[LEFT:.*]]: si64, %[[RIGHT:.*]]: si64) -> si64
// CHECK: %[[RESULT:.*]], %[[FAILED:.*]] = "sec.int.binary_checked"(%[[LEFT]], %[[RIGHT]]) <{kind = "add"}> : (si64, si64) -> (si64, i1)
// CHECK-NEXT: cf.cond_br %[[FAILED]]
// CHECK: "sec.fail.arithmetic"
// CHECK: return %[[RESULT]] : si64
// CHECK-LABEL: func.func @compare(
// CHECK-SAME: ui64
// CHECK: "sec.int.cmp"
// CHECK-SAME: (ui64, ui64) -> i1
// CHECK-LABEL: func.func @wide(
// CHECK-SAME: si128
// CHECK-SAME: ui256
