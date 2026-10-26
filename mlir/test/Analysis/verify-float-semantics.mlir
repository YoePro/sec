// RUN: sec-mlir-opt %s --sec-check-lowering-support | FileCheck %s
// RUN: sec-mlir-opt %s --sec-verify-float-semantics | FileCheck %s
// RUN: sec-mlir-opt %s --sec-verify-float-semantics --sec-lower-floats | FileCheck %s --check-prefix=LOWER
// P20-22 / package20 §§6–9,63–75,81: strict policies are preserved.

// CHECK-LABEL: func.func @float_semantics
// LOWER-LABEL: func.func @float_semantics
func.func @float_semantics(%x: f64, %y: f64) -> f64 attributes {sec.rounding = "nearest-even", sec.fast_math = "none", sec.contraction = "off", sec.denormal = "preserve"} {
  %c = "sec.const.float"() {lexeme = "1.5", bits = 4609434218613702656 : i64} : () -> f64
  %neg = "sec.float.neg"(%x) : (f64) -> f64
  %add = "sec.float.binary"(%neg, %y) {kind = "add", sec.rounding = "nearest-even"} : (f64, f64) -> f64
  %rem = "sec.float.binary"(%add, %c) {kind = "rem", sec.remainder_kind = "truncation"} : (f64, f64) -> f64
  %cmp = "sec.float.cmp"(%x, %y) {predicate = "ne"} : (f64, f64) -> i1
  %abs = "sec.float.abs"(%rem) : (f64) -> f64
  %min = "sec.float.minimum"(%abs, %y) : (f64, f64) -> f64
  %max = "sec.float.maximum"(%min, %x) : (f64, f64) -> f64
  %is = "sec.float.classify"(%max) {kind = "subnormal"} : (f64) -> i1
  %round = "sec.float.round"(%max) {kind = "nearest-even"} : (f64) -> f64
  %lo = "sec.float.property"() {kind = "negative-infinity"} : () -> f64
  %hi = "sec.float.property"() {kind = "infinity"} : () -> f64
  %v, %failed = "sec.float.clamp_checked"(%round, %lo, %hi) : (f64, f64, f64) -> (f64, i1)
  cf.cond_br %failed, ^bad, ^ok(%v : f64)
^bad:
  return %x : f64
^ok(%candidate: f64):
  return %candidate : f64
}

// CHECK-LABEL: func.func @widths
// LOWER-LABEL: func.func @widths
func.func @widths() -> f32 {
  %v = "sec.const.float"() {lexeme = "-0.0", bits = 2147483648 : i64} : () -> f32
  return %v : f32
}

// CHECK-LABEL: func.func @standard_policies
// LOWER-LABEL: func.func @standard_policies
func.func @standard_policies(%x: f32) -> f32 {
  %v = arith.addf %x, %x {fastmath = #arith.fastmath<none>, roundingmode = 0 : i32} : f32
  return %v : f32
}
