// RUN: sec-mlir-opt %s | FileCheck %s

module {
  func.func @wide_constants() attributes {sec.extern = false} {
    %i128_min = "sec.const.int"() <{value = "-170141183460469231731687303715884105728"}> : () -> si128
    %i128_max = "sec.const.int"() <{value = "170141183460469231731687303715884105727"}> : () -> si128
    %u256_max = "sec.const.int"() <{value = "115792089237316195423570985008687907853269984665640564039457584007913129639935"}> : () -> ui256
    %decimal = "sec.const.decimal"() <{coefficient = "123456789012345678901234567890", lexeme = "1234567890123456.78901234567890", scale = 14 : i32}> : () -> !sec.decimal128
    return
  }
}

// CHECK: "sec.const.int"() <{value = "-170141183460469231731687303715884105728"}> : () -> si128
// CHECK: "sec.const.int"() <{value = "170141183460469231731687303715884105727"}> : () -> si128
// CHECK: "sec.const.int"() <{value = "115792089237316195423570985008687907853269984665640564039457584007913129639935"}> : () -> ui256
// CHECK: "sec.const.decimal"() {{.*}} : () -> !sec.decimal128
