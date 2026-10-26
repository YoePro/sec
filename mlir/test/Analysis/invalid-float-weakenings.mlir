// RUN: sec-mlir-opt %s --allow-unregistered-dialect --split-input-file --sec-verify-float-semantics --verify-diagnostics
// RUN: sec-mlir-opt %s --allow-unregistered-dialect --split-input-file --sec-lower-floats --verify-diagnostics
// P20 §§7–9,81: ordinary float operations cannot introduce flush or FMA.
func.func @flush(%x: f32) -> f32 {
  // expected-error@+1 {{operation weakens ordinary Sec float semantics}}
  %v = "arith.flush_denormals"(%x) : (f32) -> f32
  return %v : f32
}
// -----
func.func @fused(%x: f32) -> f32 {
  // expected-error@+1 {{operation weakens ordinary Sec float semantics}}
  %v = "math.fma"(%x, %x, %x) : (f32, f32, f32) -> f32
  return %v : f32
}
// -----
func.func @wrong_standard_rounding(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require nearest-even rounding}}
  %v = arith.addf %x, %x downward : f32
  return %v : f32
}
