// RUN: sec-mlir-opt %s --split-input-file --verify-diagnostics

module {
  func.func @missing(%v: si8) -> si64 {
    // expected-error @+1 {{requires a sec.conversion_proof dictionary}}
    %out = "sec.numeric.convert_exact"(%v) : (si8) -> si64
    return %out : si64
  }
}

// -----
module {
  func.func @unknown(%v: si8) -> si64 {
    // expected-error @+1 {{unknown exactness proof basis}}
    %out = "sec.numeric.convert_exact"(%v) {sec.conversion_proof = {basis = "guess", plan_pointer_width = 0 : i32}} : (si8) -> si64
    return %out : si64
  }
}

// -----
module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @wrong_plan(%v: ui32) -> si64 {
    // expected-error @+1 {{exactness proof is incompatible with module plan width}}
    %out = "sec.numeric.convert_exact"(%v) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 64 : i32}} : (ui32) -> si64
    return %out : si64
  }
}

// -----
module {
  func.func @no_plan(%v: ui32) -> si64 {
    // expected-error @+1 {{plan-specific exactness proof requires module data layout}}
    %out = "sec.numeric.convert_exact"(%v) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 64 : i32}} : (ui32) -> si64
    return %out : si64
  }
}

// -----
module {
  func.func @bad_width(%v: si8) -> si64 {
    // expected-error @+1 {{exactness proof plan_pointer_width must be 0, 32 or 64 : i32}}
    %out = "sec.numeric.convert_exact"(%v) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 64 : i64}} : (si8) -> si64
    return %out : si64
  }
}

// -----

func.func @checked_failure(%a: si32) -> si8 {
  // expected-error @+1 {{failure must be OutOfRange for integer targets}}
  %c, %f = "sec.numeric.convert_checked"(%a) <{failure = "PrecisionLoss"}> : (si32) -> (si8, i1)
  return %c : si8
}

// -----

func.func @checked_float(%a: f64) -> si8 {
  // expected-error @+1 {{admits only builtin Sec integer operand and candidate types}}
  %c, %f = "sec.numeric.convert_checked"(%a) <{failure = "OutOfRange"}> : (f64) -> (si8, i1)
  return %c : si8
}
