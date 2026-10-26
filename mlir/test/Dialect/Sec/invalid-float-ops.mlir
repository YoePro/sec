// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

func.func @kind(%a: f64, %b: f64) -> f64 {
  // expected-error @+1 {{kind must be add, sub, mul, div, or rem}}
  %r = "sec.float.binary"(%a, %b) <{kind = "pow"}> : (f64, f64) -> f64
  return %r : f64
}

// -----

func.func @mixed(%a: f64, %b: f32) -> f64 {
  // expected-error @+1 {{operands and result must have the same Sec float type}}
  %r = "sec.float.binary"(%a, %b) <{kind = "add"}> : (f64, f32) -> f64
  return %r : f64
}

// -----

func.func @predicate(%a: f64, %b: f64) -> i1 {
  // expected-error @+1 {{invalid floating comparison predicate}}
  %r = "sec.float.cmp"(%a, %b) <{predicate = "one"}> : (f64, f64) -> i1
  return %r : i1
}

// -----

func.func @integer(%a: si64) -> si64 {
  // expected-error @+1 {{operand and result must have the same Sec float type}}
  %r = "sec.float.neg"(%a) : (si64) -> si64
  return %r : si64
}

// -----

func.func @wide_bits() -> f32 {
  // expected-error @+1 {{binary32 constant bits must fit in 32 bits}}
  %r = "sec.const.float"() <{bits = 4611686018427387904 : i64, lexeme = "2.0"}> : () -> f32
  return %r : f32
}

// -----

func.func @classify_kind(%a: f64) -> i1 {
  // expected-error @+1 {{kind must be finite, infinite, nan, normal, or subnormal}}
  %r = "sec.float.classify"(%a) <{kind = "zero"}> : (f64) -> i1
  return %r : i1
}

// -----

func.func @round_kind(%a: f64) -> f64 {
  // expected-error @+1 {{kind must be floor, ceiling, truncate, or nearest-even}}
  %r = "sec.float.round"(%a) <{kind = "half-up"}> : (f64) -> f64
  return %r : f64
}

// -----

func.func @property_kind() -> f64 {
  // expected-error @+1 {{kind must be min, max, epsilon, infinity, negative-infinity, or nan}}
  %r = "sec.float.property"() <{kind = "smallest"}> : () -> f64
  return %r : f64
}

// -----

func.func @minimum_mixed(%a: f64, %b: f32) -> f64 {
  // expected-error @+1 {{operands and result must have the same Sec float type}}
  %r = "sec.float.minimum"(%a, %b) : (f64, f32) -> f64
  return %r : f64
}

// -----

func.func @abs_integer(%a: si64) -> si64 {
  // expected-error @+1 {{operand and result must have the same Sec float type}}
  %r = "sec.float.abs"(%a) : (si64) -> si64
  return %r : si64
}

// -----

func.func @clamp_mixed(%a: f64, %b: f32) -> f64 {
  // expected-error @+1 {{value, bounds and result must have the same Sec float type}}
  %r, %f = "sec.float.clamp_checked"(%a, %b, %a) : (f64, f32, f64) -> (f64, i1)
  return %r : f64
}
