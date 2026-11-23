// RUN: not sec-mlir-opt --split-input-file %s 2>&1 | FileCheck %s

// P21 decimal operator verifiers (sec_mlir_dialect_v17.md §§ 6-7, 11;
// MD-066 §§ 3(2), 5; MD-067 § 7(3)).

// Multiplication prefers the sum of operand scales.
func.func @mul_scale(%a: !sec.decimal, %b: !sec.decimal) {
  %p, %f, %r = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["scale-overflow", "precision-exhausted"], kind = "mul", preferred_scale = "max-operand-scale", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
  return
}
// CHECK: preferred_scale must be sum-of-operand-scales

// -----

// Only division rounds its quotient.
func.func @add_rounds(%a: !sec.decimal, %b: !sec.decimal) {
  %p, %f, %r = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["scale-overflow"], kind = "add", preferred_scale = "max-operand-scale", quotient = "nearest-even", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
  return
}
// CHECK: only div rounds its quotient

// -----

// Division must account for a runtime zero divisor.
func.func @div_zero(%a: !sec.decimal, %b: !sec.decimal) {
  %p, %f, %r = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["scale-overflow"], kind = "div", preferred_scale = "max-operand-scale", quotient = "nearest-even", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
  return
}
// CHECK: div and rem must list division-by-zero

// -----

// A rounded quotient is a success: precision-loss is no decimal reason.
func.func @precision_loss(%a: !sec.decimal, %b: !sec.decimal) {
  %p, %f, %r = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["division-by-zero", "precision-loss"], kind = "div", preferred_scale = "max-operand-scale", quotient = "nearest-even", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
  return
}
// CHECK: invalid decimal failure reason precision-loss

// -----

// The coefficient width follows the decimal type.
func.func @width(%a: !sec.decimal128) {
  %p, %f, %r = "sec.decimal.unary"(%a) <{coefficient_bits = 64 : i32, failures = ["scale-overflow"], kind = "neg", preferred_scale = "operand-scale", scale_policy = "nearest-exact"}> : (!sec.decimal128) -> (!sec.decimal128, i1, !sec.arithmetic_failure_reason)
  return
}
// CHECK: coefficient_bits must match the decimal type

// -----

// No binary-float decimal backend.
func.func @float(%a: f64, %b: f64) {
  %c = "sec.decimal.cmp"(%a, %b) <{predicate = "lt"}> : (f64, f64) -> i1
  return
}
// CHECK: operands must share one primitive decimal type

// -----

// No implicit widening between decimal kinds.
func.func @mixed(%a: !sec.decimal, %b: !sec.decimal128) {
  %p, %f, %r = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["scale-overflow"], kind = "sub", preferred_scale = "max-operand-scale", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal128) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
  return
}
// CHECK: operands must have one decimal type
