// RUN: not sec-mlir-opt --split-input-file %s 2>&1 | FileCheck %s

// Failures out of P20 priority order (§ 41).
func.func @order(%x: f64) {
  %c, %failed, %a, %b = "sec.numeric.convert_checked"(%x) <{failures = ["OutOfRange", "NonFinite"]}> : (f64) -> (si32, i1, i1, i1)
  return
}
// CHECK: failures must be distinct NonFinite, OutOfRange, PrecisionLoss in priority order

// -----

// One reason flag per listed failure.
func.func @flags(%x: f64) {
  %c, %failed, %a = "sec.numeric.convert_checked"(%x) <{failures = ["OutOfRange", "PrecisionLoss"]}> : (f64) -> (f32, i1, i1)
  return
}
// CHECK: needs one reason flag per listed failure

// -----

// The float form needs a float on one side.
func.func @integers(%x: si64) {
  %c, %failed, %a = "sec.numeric.convert_checked"(%x) <{failures = ["OutOfRange"]}> : (si64) -> (si32, i1, i1)
  return
}
// CHECK: one of them a Sec float

// -----

// The integer form has no reason flags.
func.func @integer_reasons(%x: si64) {
  %c, %failed, %a = "sec.numeric.convert_checked"(%x) <{failure = "OutOfRange"}> : (si64) -> (si32, i1, i1)
  return
}
// CHECK: the integer form has no reason flags

// -----

func.func @to_bool(%x: si64) {
  %y = "sec.float.to_bool"(%x) : (si64) -> i1
  return
}
// CHECK: operand must be a Sec float type

// -----

// A non-rune integer target fails as OutOfRange, never InvalidScalar.
func.func @invalid_scalar_for_integer(%x: si64) {
  %c, %failed = "sec.numeric.convert_checked"(%x) <{failure = "InvalidScalar"}> : (si64) -> (si32, i1)
  return
}
// CHECK: InvalidScalar for rune targets

// -----

// A rune target fails as InvalidScalar.
func.func @out_of_range_for_rune(%x: ui32) {
  %c, %failed = "sec.numeric.convert_checked"(%x) <{failure = "OutOfRange"}> : (ui32) -> (!sec.rune, i1)
  return
}
// CHECK: InvalidScalar for rune targets
