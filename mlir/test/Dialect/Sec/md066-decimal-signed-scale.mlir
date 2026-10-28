// RUN: sec-mlir-opt %s | FileCheck %s

// MD-066: the decimal scale is a signed int32 over its whole domain, so a
// negative scale (coefficient × 10^3 here) is a valid constant.
module {
  func.func @signed_scales() -> (!sec.decimal, !sec.decimal) {
    %large = "sec.const.decimal"() <{coefficient = "1000000000000000000", lexeme = "1000000000000000000e12", scale = -12 : i32}> : () -> !sec.decimal
    %fraction = "sec.const.decimal"() <{coefficient = "125", lexeme = "0.125", scale = 3 : i32}> : () -> !sec.decimal
    return %large, %fraction : !sec.decimal, !sec.decimal
  }
}

// CHECK: scale = -12 : i32
// CHECK: scale = 3 : i32
