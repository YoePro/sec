// RUN: sec-mlir-opt %s --split-input-file --sec-verify-float-semantics --verify-diagnostics
// RUN: sec-mlir-opt %s --split-input-file --sec-lower-floats --verify-diagnostics
// P20 §§18,64–69,74–76,81: reject contradictory serialized scalar/predicate facts.
func.func @valid(%x: f32, %y: f32) -> i1 {
  %r = "sec.float.binary"(%x, %y) {kind = "rem", sec.float_format = 32 : i32, sec.float_source_scalar = "float32", sec.rounding = "nearest-even", sec.fast_math = "none", sec.contraction = "off", sec.denormal = "preserve", sec.remainder_kind = "truncation"} : (f32, f32) -> f32
  %c = "sec.float.cmp"(%r, %y) {predicate = "ne", sec.comparison_predicate = "une", sec.float_format = 32 : i32, sec.float_source_scalar = "float"} : (f32, f32) -> i1
  return %c : i1
}
// -----
func.func @wrong_format(%x: f32) -> f32 {
  // expected-error@+1 {{float format fact disagrees with scalar type}}
  %r = "sec.float.abs"(%x) {sec.float_format = 64 : i32} : (f32) -> f32
  return %r : f32
}
// -----
func.func @wrong_format_type(%x: f32) -> f32 {
  // expected-error@+1 {{invalid explicit float format fact}}
  %r = "sec.float.abs"(%x) {sec.float_format = "binary32"} : (f32) -> f32
  return %r : f32
}
// -----
func.func @wrong_scalar(%x: f32) -> f32 {
  // expected-error@+1 {{float source scalar fact disagrees with scalar type}}
  %r = "sec.float.abs"(%x) {sec.float_source_scalar = "float64"} : (f32) -> f32
  return %r : f32
}
// -----
func.func @wrong_predicate(%x: f32) -> i1 {
  // expected-error@+1 {{float comparison predicate fact disagrees with source predicate}}
  %r = "sec.float.cmp"(%x, %x) {predicate = "ne", sec.comparison_predicate = "one"} : (f32, f32) -> i1
  return %r : i1
}
// -----
func.func @irrelevant_predicate(%x: f32) -> f32 {
  // expected-error@+1 {{invalid explicit float comparison predicate fact}}
  %r = "sec.float.abs"(%x) {sec.comparison_predicate = "oeq"} : (f32) -> f32
  return %r : f32
}
// -----
func.func @irrelevant_remainder(%x: f32) -> f32 {
  // expected-error@+1 {{remainder fact requires float rem operation}}
  %r = "sec.float.abs"(%x) {sec.remainder_kind = "truncation"} : (f32) -> f32
  return %r : f32
}
// -----
func.func @wrong_platform_identity(%x: !sec.float) -> !sec.float {
  // expected-error@+1 {{float source scalar fact loses platform scalar identity}}
  %r = "sec.float.abs"(%x) {sec.float_source_scalar = "float64"} : (!sec.float) -> !sec.float
  return %r : !sec.float
}
