// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module attributes {sec.dialect_version = 1 : i32} {
  func.func @identity(%value: !sec.distinct<"main::CustomerID", i64>)
      -> !sec.distinct<"main::CustomerID", i64> {
    return %value : !sec.distinct<"main::CustomerID", i64>
  }
}

// CHECK: !sec.distinct<"main::CustomerID", i64>
