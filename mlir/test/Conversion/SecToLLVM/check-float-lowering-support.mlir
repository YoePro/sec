// RUN: sec-mlir-opt %s --split-input-file --sec-check-lowering-support --verify-diagnostics
// RUN: sec-mlir-opt %s --split-input-file --sec-lower-to-llvm=entry= --verify-diagnostics
// P20-28: incomplete materialization fails at track entry, with no fallback.
func.func @legacy_literal() -> f64 {
  // expected-error@+1 {{[unsupported-lowering] sec.const.float requires canonical bits before native lowering}}
  %v = "sec.const.float"() {lexeme = "1.25"} : () -> f64
  return %v : f64
}
// -----
func.func @decimal_is_not_binary_float() -> !sec.decimal {
  // expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower 'sec.const.decimal' yet}}
  %v = "sec.const.decimal"() {coefficient = "125", scale = 2 : i32, lexeme = "1.25"} : () -> !sec.decimal
  return %v : !sec.decimal
}
// -----
func.func @decimal128_is_not_binary_float() -> !sec.decimal128 {
  // expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower 'sec.const.decimal' yet}}
  %v = "sec.const.decimal"() {coefficient = "125", scale = 2 : i32, lexeme = "1.25"} : () -> !sec.decimal128
  return %v : !sec.decimal128
}
// -----
func.func @float_conversion_not_admitted(%x: f32) -> f64 {
  // expected-error@+1 {{schema 11 admits only builtin Sec integer operand and result types}}
  %v = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (f32) -> f64
  return %v : f64
}
// -----
func.func @checked_float_conversion_not_admitted(%x: f64) -> si32 {
  // expected-error@+1 {{admits only builtin Sec integer operand and candidate types}}
  %v, %failed = "sec.numeric.convert_checked"(%x) {failure = "OutOfRange"} : (f64) -> (si32, i1)
  return %v : si32
}
// -----
func.func @float_to_bool_not_admitted(%x: f64) -> i1 {
  // expected-error@+1 {{schema 11 admits only builtin Sec integer operand and result types}}
  %v = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (f64) -> i1
  return %v : i1
}
