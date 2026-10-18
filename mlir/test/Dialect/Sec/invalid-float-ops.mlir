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
