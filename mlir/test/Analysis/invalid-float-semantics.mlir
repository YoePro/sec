// RUN: sec-mlir-opt %s --split-input-file --sec-lower-floats --verify-diagnostics
// RUN: sec-mlir-opt %s --split-input-file --sec-verify-float-semantics --verify-diagnostics
// -----
func.func @bad_policy_0(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require fast-math none}}
  %v = "sec.float.neg"(%x) {sec.fast_math = "fast"} : (f32) -> f32
  return %v : f32
}
// -----
func.func @bad_policy_1(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require nearest-even rounding}}
  %v = "sec.float.neg"(%x) {sec.rounding = "toward-zero"} : (f32) -> f32
  return %v : f32
}
// -----
func.func @bad_policy_2(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics forbid contraction}}
  %v = "sec.float.neg"(%x) {sec.contraction = true} : (f32) -> f32
  return %v : f32
}
// -----
func.func @bad_policy_3(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require subnormal preservation}}
  %v = "sec.float.neg"(%x) {sec.denormal = "flush"} : (f32) -> f32
  return %v : f32
}
// -----
func.func @bad_policy_4(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require truncation remainder}}
  %v = "sec.float.neg"(%x) {sec.remainder_kind = "ieee"} : (f32) -> f32
  return %v : f32
}
// -----
func.func @bad_policy_5(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require fast-math none}}
  %v = "sec.float.neg"(%x) {sec.fastmath = #arith.fastmath<nnan>} : (f32) -> f32
  return %v : f32
}
// -----
func.func @bad_policy_6(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require nearest-even rounding}}
  %v = "sec.float.neg"(%x) {sec.roundingmode = 3 : i32} : (f32) -> f32
  return %v : f32
}
// -----
// expected-error@+1 {{Sec float semantics require subnormal preservation}}
module attributes {sec.denormal = "flush"} {
  func.func @inherited() { return }
}
// -----
func.func @clamp_without_guard(%x: f32) -> f32 {
  // expected-error@+1 {{Clamp requires an immediate single-use failed branch}}
  %v, %failed = "sec.float.clamp_checked"(%x, %x, %x) : (f32, f32, f32) -> (f32, i1)
  return %v : f32
}
// -----
func.func @clamp_failure_leak(%x: f32) -> f32 {
  // expected-error@+1 {{Clamp candidate must be confined to the success edge}}
  %v, %failed = "sec.float.clamp_checked"(%x, %x, %x) : (f32, f32, f32) -> (f32, i1)
  cf.cond_br %failed, ^bad(%v : f32), ^ok
^bad(%leak: f32):
  return %leak : f32
^ok:
  return %x : f32
}
// -----
func.func @clamp_merge_leak(%x: f32) -> f32 {
  // expected-error@+1 {{Clamp candidate must be confined to the success edge}}
  %v, %failed = "sec.float.clamp_checked"(%x, %x, %x) : (f32, f32, f32) -> (f32, i1)
  cf.cond_br %failed, ^bad, ^ok
^bad:
  cf.br ^merge
^ok:
  cf.br ^merge
^merge:
  return %v : f32
}
// -----
func.func @invalid_format() -> f16 {
  // expected-error@+1 {{result must be !sec.float, f32, or f64}}
  %v = "sec.const.float"() {lexeme = "1", bits = 0 : i64} : () -> f16
  return %v : f16
}
// -----
func.func @oversized_binary32() -> f32 {
  // expected-error@+1 {{binary32 constant bits must fit in 32 bits}}
  %v = "sec.const.float"() {lexeme = "1", bits = 4294967296 : i64} : () -> f32
  return %v : f32
}
// -----
func.func @invalid_predicate(%x: f32) -> i1 {
  // expected-error@+1 {{invalid floating comparison predicate}}
  %v = "sec.float.cmp"(%x, %x) {predicate = "unordered"} : (f32, f32) -> i1
  return %v : i1
}
// -----
func.func @invalid_remainder(%x: f32) -> f32 {
  // expected-error@+1 {{kind must be add, sub, mul, div, or rem}}
  %v = "sec.float.binary"(%x, %x) {kind = "ieee-remainder"} : (f32, f32) -> f32
  return %v : f32
}
// -----
func.func @invalid_classification(%x: f32) -> i1 {
  // expected-error@+1 {{kind must be finite, infinite, nan, normal, or subnormal}}
  %v = "sec.float.classify"(%x) {kind = "sign"} : (f32) -> i1
  return %v : i1
}
// -----
func.func @invalid_round(%x: f32) -> f32 {
  // expected-error@+1 {{kind must be floor, ceiling, truncate, or nearest-even}}
  %v = "sec.float.round"(%x) {kind = "away"} : (f32) -> f32
  return %v : f32
}
// -----
func.func @invalid_property() -> f32 {
  // expected-error@+1 {{kind must be min, max, epsilon, infinity, negative-infinity, or nan}}
  %v = "sec.float.property"() {kind = "lowest-positive"} : () -> f32
  return %v : f32
}
// -----
func.func @invalid_clamp_types(%x: f32, %y: f64) -> f32 {
  // expected-error@+1 {{value, bounds and result must have the same Sec float type}}
  %v, %failed = "sec.float.clamp_checked"(%x, %y, %x) : (f32, f64, f32) -> (f32, i1)
  return %v : f32
}
// -----
func.func @same_clamp_successor(%x: f32) -> f32 {
  // expected-error@+1 {{Clamp requires an immediate single-use failed branch with distinct outcomes}}
  %v, %failed = "sec.float.clamp_checked"(%x, %x, %x) : (f32, f32, f32) -> (f32, i1)
  cf.cond_br %failed, ^merge, ^merge
^merge:
  return %x : f32
}
// -----
func.func @malformed_policy(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require nearest-even rounding}}
  %v = "sec.float.neg"(%x) {sec.rounding = true} : (f32) -> f32
  return %v : f32
}
// -----
func.func @missing_bits() -> f64 {
  // expected-error@+1 {{P20 float constants require canonical bits}}
  %v = "sec.const.float"() {lexeme = "1"} : () -> f64
  return %v : f64
}
// -----
func.func @weakened_standard_arithmetic(%x: f32) -> f32 {
  // expected-error@+1 {{Sec float semantics require fast-math none}}
  %v = arith.addf %x, %x fastmath<contract> : f32
  return %v : f32
}
