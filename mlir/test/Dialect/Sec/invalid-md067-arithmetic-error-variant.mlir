// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

// MD-067: no variant beyond the five canonical ArithmeticError variants.
module {
  func.func @unknown(%failure: !sec.core_error<"core::ArithmeticError">) -> i1 {
    %bad = "sec.core_error.is_variant"(%failure) <{variant = "PrecisionLoss"}> : (!sec.core_error<"core::ArithmeticError">) -> i1
    return %bad : i1
  }
}

// CHECK: unknown ArithmeticError variant
