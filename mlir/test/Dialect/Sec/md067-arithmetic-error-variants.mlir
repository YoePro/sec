// RUN: sec-mlir-opt %s | FileCheck %s

// MD-067: core::ArithmeticError has exactly five variants; the decimal
// variants ScaleOverflow and PrecisionExhausted are valid variant tests.
module {
  func.func @decimal_variants(%failure: !sec.core_error<"core::ArithmeticError">) -> (i1, i1) {
    %scale = "sec.core_error.is_variant"(%failure) <{variant = "ScaleOverflow"}> : (!sec.core_error<"core::ArithmeticError">) -> i1
    %digits = "sec.core_error.is_variant"(%failure) <{variant = "PrecisionExhausted"}> : (!sec.core_error<"core::ArithmeticError">) -> i1
    return %scale, %digits : i1, i1
  }
}

// CHECK: variant = "ScaleOverflow"
// CHECK: variant = "PrecisionExhausted"
