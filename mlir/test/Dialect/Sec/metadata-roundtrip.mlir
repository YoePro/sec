// RUN: sec-mlir-opt %s | FileCheck %s

module attributes {sec.dialect_version = 1 : i32} {
  func.func @metadata(%value: !sec.named<"main::Speed", i64>)
      -> !sec.named<"main::Speed", i64>
      attributes {
        sec.layout_ref = "layout:main::Speed",
        sec.symbol_id = "main::metadata#0",
        sec.synthesized = true,
        sec.type_id = "main::Speed"
      } {
    return %value : !sec.named<"main::Speed", i64>
  }
}

// CHECK: sec.dialect_version = 1 : i32
// CHECK-DAG: sec.layout_ref = "layout:main::Speed"
// CHECK-DAG: sec.symbol_id = "main::metadata#0"
// CHECK-DAG: sec.synthesized = true
// CHECK-DAG: sec.type_id = "main::Speed"
