// RUN: sec-mlir-opt %s | FileCheck %s

// P20 conversions (sec-mlir-dialect_package20.md §§ 55, 70-72): exact float
// conversions with their proof, float-to-bool, and the checked float form with
// its ordered failures and one reason flag each.
module {
  func.func @widen(%x: f32) -> f64 attributes {sec.function_id = "main::Widen(float32)", sec.source_name = "Widen", sec.extern = false, sec.unsafe = false} {
    %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (f32) -> f64
    return %y : f64
  }
  func.func @exact(%x: si16) -> f32 attributes {sec.function_id = "main::Exact(int16)", sec.source_name = "Exact", sec.extern = false, sec.unsafe = false} {
    %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "value-interval", plan_pointer_width = 0 : i32}} : (si16) -> f32
    return %y : f32
  }
  func.func @truth(%x: f64) -> i1 attributes {sec.function_id = "main::Truth(float64)", sec.source_name = "Truth", sec.extern = false, sec.unsafe = false} {
    %y = "sec.float.to_bool"(%x) : (f64) -> i1
    return %y : i1
  }
  func.func @checked(%x: f64) -> si32 attributes {sec.function_id = "main::Checked(float64)", sec.source_name = "Checked", sec.extern = false, sec.unsafe = false} {
    %c, %failed, %nf, %range, %precision = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite", "OutOfRange", "PrecisionLoss"]}> : (f64) -> (si32, i1, i1, i1, i1)
    return %c : si32
  }
}

// CHECK: sec.numeric.convert_exact{{.*}}(f32) -> f64
// CHECK: sec.numeric.convert_exact{{.*}}basis = "value-interval"{{.*}}(si16) -> f32
// CHECK: sec.float.to_bool
// CHECK: failures = ["NonFinite", "OutOfRange", "PrecisionLoss"]
