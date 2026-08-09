// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 4 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "checked-integers",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @all(%signed: si128, %unsigned: ui256, %count: si32) {
    %plus = "sec.int.unary_plus"(%signed) : (si128) -> si128
    %neg, %neg_failed = "sec.int.neg_checked"(%signed) : (si128) -> (si128, i1)
    %sum, %sum_failed = "sec.int.binary_checked"(%signed, %signed) <{kind = "add"}> : (si128, si128) -> (si128, i1)
    %not = "sec.int.bit_not"(%unsigned) : (ui256) -> ui256
    %bits = "sec.int.bitwise"(%unsigned, %unsigned) <{kind = "xor"}> : (ui256, ui256) -> ui256
    %shifted, %shift_failed = "sec.int.shift_checked"(%unsigned, %count) <{kind = "right_unsigned"}> : (ui256, si32) -> (ui256, i1)
    %equal = "sec.int.cmp"(%signed, %signed) <{predicate = "eq"}> : (si128, si128) -> i1
    return
  }

  func.func @failure() {
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  }

  func.func @binary_kinds(%signed: si64, %unsigned: ui64) {
    %add, %add_failed = "sec.int.binary_checked"(%signed, %signed) <{kind = "add"}> : (si64, si64) -> (si64, i1)
    %subtract, %subtract_failed = "sec.int.binary_checked"(%unsigned, %unsigned) <{kind = "subtract"}> : (ui64, ui64) -> (ui64, i1)
    %multiply, %multiply_failed = "sec.int.binary_checked"(%signed, %signed) <{kind = "multiply"}> : (si64, si64) -> (si64, i1)
    %divide, %divide_failed = "sec.int.binary_checked"(%unsigned, %unsigned) <{kind = "divide"}> : (ui64, ui64) -> (ui64, i1)
    %remainder, %remainder_failed = "sec.int.binary_checked"(%signed, %signed) <{kind = "remainder"}> : (si64, si64) -> (si64, i1)
    return
  }

  func.func @bitwise_kinds(%value: ui32) {
    %and = "sec.int.bitwise"(%value, %value) <{kind = "and"}> : (ui32, ui32) -> ui32
    %or = "sec.int.bitwise"(%value, %value) <{kind = "or"}> : (ui32, ui32) -> ui32
    %xor = "sec.int.bitwise"(%value, %value) <{kind = "xor"}> : (ui32, ui32) -> ui32
    return
  }

  func.func @shift_kinds(%signed: si256, %unsigned: ui128, %count: ui8) {
    %ls, %ls_failed = "sec.int.shift_checked"(%signed, %count) <{kind = "left_signed"}> : (si256, ui8) -> (si256, i1)
    %rs, %rs_failed = "sec.int.shift_checked"(%signed, %count) <{kind = "right_signed"}> : (si256, ui8) -> (si256, i1)
    %lu, %lu_failed = "sec.int.shift_checked"(%unsigned, %count) <{kind = "left_unsigned"}> : (ui128, ui8) -> (ui128, i1)
    %ru, %ru_failed = "sec.int.shift_checked"(%unsigned, %count) <{kind = "right_unsigned"}> : (ui128, ui8) -> (ui128, i1)
    return
  }

  func.func @comparison_predicates(%signed: si16) {
    %eq = "sec.int.cmp"(%signed, %signed) <{predicate = "eq"}> : (si16, si16) -> i1
    %ne = "sec.int.cmp"(%signed, %signed) <{predicate = "ne"}> : (si16, si16) -> i1
    %lt = "sec.int.cmp"(%signed, %signed) <{predicate = "lt"}> : (si16, si16) -> i1
    %le = "sec.int.cmp"(%signed, %signed) <{predicate = "le"}> : (si16, si16) -> i1
    %gt = "sec.int.cmp"(%signed, %signed) <{predicate = "gt"}> : (si16, si16) -> i1
    %ge = "sec.int.cmp"(%signed, %signed) <{predicate = "ge"}> : (si16, si16) -> i1
    return
  }

  func.func @active_widths(
      %si8: si8, %si16: si16, %si32: si32, %si64: si64, %si128: si128, %si256: si256,
      %ui8: ui8, %ui16: ui16, %ui32: ui32, %ui64: ui64, %ui128: ui128, %ui256: ui256) {
    %s8 = "sec.int.unary_plus"(%si8) : (si8) -> si8
    %s16 = "sec.int.unary_plus"(%si16) : (si16) -> si16
    %s32 = "sec.int.unary_plus"(%si32) : (si32) -> si32
    %s64 = "sec.int.unary_plus"(%si64) : (si64) -> si64
    %s128 = "sec.int.unary_plus"(%si128) : (si128) -> si128
    %s256 = "sec.int.unary_plus"(%si256) : (si256) -> si256
    %u8 = "sec.int.unary_plus"(%ui8) : (ui8) -> ui8
    %u16 = "sec.int.unary_plus"(%ui16) : (ui16) -> ui16
    %u32 = "sec.int.unary_plus"(%ui32) : (ui32) -> ui32
    %u64 = "sec.int.unary_plus"(%ui64) : (ui64) -> ui64
    %u128 = "sec.int.unary_plus"(%ui128) : (ui128) -> ui128
    %u256 = "sec.int.unary_plus"(%ui256) : (ui256) -> ui256
    return
  }
}

// CHECK: sec.dialect_version = 4 : i32
// CHECK: "sec.int.unary_plus"
// CHECK: "sec.int.neg_checked"
// CHECK: "sec.int.binary_checked"
// CHECK: kind = "add"
// CHECK: "sec.int.bit_not"
// CHECK: "sec.int.bitwise"
// CHECK: "sec.int.shift_checked"
// CHECK: "sec.int.cmp"
// CHECK: "sec.fail.arithmetic"
// CHECK: sec.operator = "+"
// CHECK: kind = "subtract"
// CHECK: kind = "multiply"
// CHECK: kind = "divide"
// CHECK: kind = "remainder"
// CHECK: kind = "and"
// CHECK: kind = "or"
// CHECK: kind = "xor"
// CHECK: kind = "left_signed"
// CHECK: kind = "right_signed"
// CHECK: kind = "left_unsigned"
// CHECK: predicate = "ne"
// CHECK: predicate = "lt"
// CHECK: predicate = "le"
// CHECK: predicate = "gt"
// CHECK: predicate = "ge"
// CHECK-LABEL: func.func @active_widths
// CHECK-SAME: si8
// CHECK-SAME: si16
// CHECK-SAME: si32
// CHECK-SAME: si64
// CHECK-SAME: si128
// CHECK-SAME: si256
// CHECK-SAME: ui8
// CHECK-SAME: ui16
// CHECK-SAME: ui32
// CHECK-SAME: ui64
// CHECK-SAME: ui128
// CHECK-SAME: ui256
