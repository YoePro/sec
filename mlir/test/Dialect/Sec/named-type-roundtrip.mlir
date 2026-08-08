// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module attributes {sec.dialect_version = 1 : i32} {
  func.func @identity(%value: !sec.named<"main::Speed", i64>)
      -> !sec.named<"main::Speed", i64> {
    return %value : !sec.named<"main::Speed", i64>
  }

  func.func @floating(%value: !sec.named<"main::Ratio", f64>)
      -> !sec.named<"main::Ratio", f64> {
    return %value : !sec.named<"main::Ratio", f64>
  }
}

// CHECK: !sec.named<"main::Speed", i64>
// CHECK: !sec.named<"main::Ratio", f64>
