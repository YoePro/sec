// RUN: sec-mlir-opt --sec-lower-checked-integers %s | FileCheck %s

module {
  func.func @signed_add(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : si32
  }

  func.func @unsigned_subtract(%left: ui64, %right: ui64) -> ui64 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "subtract"}> : (ui64, ui64) -> (ui64, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "-"} : () -> ()
  ^success:
    return %result : ui64
  }

  func.func @signed_multiply(%left: si128, %right: si128) -> si128 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "multiply"}> : (si128, si128) -> (si128, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "*"} : () -> ()
  ^success:
    return %result : si128
  }

  func.func @signed_divide(%left: si64, %right: si64) -> si64 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "divide"}> : (si64, si64) -> (si64, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "division"}> {sec.operator = "/"} : () -> ()
  ^success:
    return %result : si64
  }

  func.func @unsigned_remainder(%left: ui256, %right: ui256) -> ui256 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "remainder"}> : (ui256, ui256) -> (ui256, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "remainder"}> {sec.operator = "%"} : () -> ()
  ^success:
    return %result : ui256
  }

  func.func @negate(%value: si16) -> si16 {
    %result, %failed = "sec.int.neg_checked"(%value) : (si16) -> (si16, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "-"} : () -> ()
  ^success:
    return %result : si16
  }

  func.func @signed_left(%value: si256, %count: si8) -> si256 {
    %result, %failed = "sec.int.shift_checked"(%value, %count) <{kind = "left_signed"}> : (si256, si8) -> (si256, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "shift"}> {sec.operator = "<<"} : () -> ()
  ^success:
    return %result : si256
  }

  func.func @unsigned_right(%value: ui128, %count: ui16) -> ui128 {
    %result, %failed = "sec.int.shift_checked"(%value, %count) <{kind = "right_unsigned"}> : (ui128, ui16) -> (ui128, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "shift"}> {sec.operator = ">>"} : () -> ()
  ^success:
    return %result : ui128
  }

  func.func @total_ops(%signed: si8, %unsigned: ui8) -> i1 {
    %plus = "sec.int.unary_plus"(%signed) : (si8) -> si8
    %not = "sec.int.bit_not"(%unsigned) : (ui8) -> ui8
    %and = "sec.int.bitwise"(%not, %unsigned) <{kind = "and"}> : (ui8, ui8) -> ui8
    %cmp = "sec.int.cmp"(%plus, %signed) <{predicate = "lt"}> : (si8, si8) -> i1
    return %cmp : i1
  }
}

// CHECK-LABEL: func.func @signed_add(%{{.*}}: i32, %{{.*}}: i32) -> i32
// CHECK: arith.extsi
// CHECK: arith.addi
// CHECK: arith.cmpi slt
// CHECK: arith.cmpi sgt
// CHECK: cf.cond_br
// CHECK: "sec.fail.arithmetic"
// CHECK-LABEL: func.func @unsigned_subtract(%{{.*}}: i64, %{{.*}}: i64) -> i64
// CHECK: arith.subi
// CHECK: arith.cmpi ult
// CHECK-LABEL: func.func @signed_multiply(%{{.*}}: i128, %{{.*}}: i128) -> i128
// CHECK: arith.muli {{.*}} : i256
// CHECK-LABEL: func.func @signed_divide
// CHECK: %[[FAILED:.*]] = arith.ori
// CHECK: %[[SAFE:.*]] = arith.select %[[FAILED]]
// CHECK: arith.divsi {{.*}}, %[[SAFE]]
// CHECK-LABEL: func.func @unsigned_remainder
// CHECK: %[[ZERO:.*]] = arith.cmpi eq
// CHECK: %[[RSAFE:.*]] = arith.select %[[ZERO]]
// CHECK: arith.remui {{.*}}, %[[RSAFE]]
// CHECK-LABEL: func.func @negate
// CHECK: arith.cmpi eq
// CHECK: arith.subi
// CHECK-LABEL: func.func @signed_left
// CHECK: arith.select
// CHECK: arith.shli {{.*}} : i512
// CHECK: arith.cmpi slt
// CHECK: arith.cmpi sgt
// CHECK-LABEL: func.func @unsigned_right
// CHECK: arith.select
// CHECK: arith.shrui
// CHECK-LABEL: func.func @total_ops
// CHECK: arith.xori
// CHECK: arith.andi
// CHECK: arith.cmpi slt
// CHECK-NOT: "sec.int.
// CHECK-NOT: overflow<
// CHECK-NOT: nneg
// CHECK-NOT: exact
// CHECK-NOT: llvm.
