// RUN: sec-mlir-opt --sec-resolve-scalar-layout %s | FileCheck %s
// RUN: sec-mlir-opt --sec-verify-checked-integer-guards --sec-lower-scalar-core --sec-verify-checked-integer-guards %s -o /dev/null

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>,
  sec.dialect_version = 4 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "checked-layout-32",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "armv7",
  sec.target_triple = "armv7-unknown-linux-gnueabihf",
  sec.target_abi = "gnueabihf",
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
    %result = "sec.int.cmp"(%left, %right) <{predicate = "ge"}> : (!sec.uint, !sec.uint) -> i1
    return %result : i1
  }

  func.func @wide(%signed: si256, %unsigned: ui128) {
    %signed_copy = "sec.int.unary_plus"(%signed) : (si256) -> si256
    %unsigned_copy = "sec.int.unary_plus"(%unsigned) : (ui128) -> ui128
    return
  }
}

// CHECK-LABEL: func.func @add(
// CHECK-SAME: si32
// CHECK: "sec.int.binary_checked"
// CHECK-SAME: (si32, si32) -> (si32, i1)
// CHECK-LABEL: func.func @compare(
// CHECK-SAME: ui32
// CHECK: "sec.int.cmp"
// CHECK-SAME: (ui32, ui32) -> i1
// CHECK-LABEL: func.func @wide(
// CHECK-SAME: si256
// CHECK-SAME: ui128
