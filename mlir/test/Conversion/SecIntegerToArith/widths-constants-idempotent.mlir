// RUN: sec-mlir-opt --sec-lower-checked-integers --sec-lower-checked-integers %s | FileCheck %s

module {
  func.func @widths(%s8: si8, %s16: si16, %s32: si32, %s64: si64, %s128: si128, %s256: si256,
                    %u8: ui8, %u16: ui16, %u32: ui32, %u64: ui64, %u128: ui128, %u256: ui256) {
    return
  }

  func.func @signed_constant() -> si128 {
    %value = "sec.const.int"() <{value = "-170141183460469231731687303715884105728"}> : () -> si128
    return %value : si128
  }

  func.func @unsigned_constant() -> ui256 {
    %value = "sec.const.int"() <{value = "115792089237316195423570985008687907853269984665640564039457584007913129639935"}> : () -> ui256
    return %value : ui256
  }
}

// CHECK-LABEL: func.func @widths(
// CHECK-SAME: i8
// CHECK-SAME: i16
// CHECK-SAME: i32
// CHECK-SAME: i64
// CHECK-SAME: i128
// CHECK-SAME: i256
// CHECK-LABEL: func.func @signed_constant() -> i128
// CHECK: arith.constant -170141183460469231731687303715884105728 : i128
// CHECK-LABEL: func.func @unsigned_constant() -> i256
// CHECK: arith.constant -1 : i256
// CHECK-NOT: sec.const.int
// CHECK-NOT: sec.int.
// CHECK-NOT: unrealized_conversion_cast
