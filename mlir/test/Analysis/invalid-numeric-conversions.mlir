// RUN: sec-mlir-opt %s --split-input-file --sec-verify-numeric-conversions --verify-diagnostics

func.func @narrow_by_domain(%x: f64) -> f32 {
  // expected-error@+1 {{a narrowing float conversion is exact only by a constant proof}}
  %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (f64) -> f32
  return %y : f32
}

// -----

func.func @float_to_int_by_domain(%x: f64) -> si32 {
  // expected-error@+1 {{a float-to-integer conversion is exact only by a constant proof}}
  %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (f64) -> si32
  return %y : si32
}

// -----

func.func @wide_int_by_domain(%x: si64) -> f64 {
  // expected-error@+1 {{an integer-to-float conversion wider than the significand needs a value proof}}
  %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (si64) -> f64
  return %y : f64
}

// -----

func.func @candidate_on_failure(%x: f64) -> si32 {
  // expected-error@+1 {{candidate must be confined to the success edge}}
  %c, %failed, %nf = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite"]}> : (f64) -> (si32, i1, i1)
  cf.cond_br %failed, ^failure, ^success
^failure:
  return %c : si32
^success:
  return %c : si32
}

// -----

func.func @reason_on_success(%x: f64) -> i1 {
  // expected-error@+1 {{reason flags may only be used on the failure edge}}
  %c, %failed, %nf = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite"]}> : (f64) -> (si32, i1, i1)
  cf.cond_br %failed, ^failure, ^success
^failure:
  %t = arith.constant true
  return %t : i1
^success:
  return %nf : i1
}

// -----

func.func @wrong_order(%x: f64) -> si32 {
  // expected-error@+1 {{the failure edge must test the reasons in priority order}}
  %c, %failed, %nf, %range = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite", "OutOfRange"]}> : (f64) -> (si32, i1, i1, i1)
  cf.cond_br %failed, ^failure, ^success
^failure:
  cf.cond_br %range, ^a, ^b
^a:
  %z = "sec.const.int"() <{value = "0"}> : () -> si32
  return %z : si32
^b:
  %w = "sec.const.int"() <{value = "1"}> : () -> si32
  return %w : si32
^success:
  return %c : si32
}

// -----

func.func @wrong_variant(%x: f64) -> si32 {
  // expected-error@+1 {{reason NonFinite maps to ConversionError.OutOfRange}}
  %c, %failed, %nf = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite"]}> : (f64) -> (si32, i1, i1)
  cf.cond_br %failed, ^failure, ^success
^failure:
  %e = "sec.union.construct"() <{fieldNames = [], payloadActions = [], variant = 0 : i32}> : () -> !sec.union<identity = "core::ConversionError", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "OutOfRange", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "NonFinite", kind = "empty", fields = []>]>
  %z = "sec.const.int"() <{value = "0"}> : () -> si32
  return %z : si32
^success:
  return %c : si32
}

// -----

func.func @wrong_family(%x: f64) -> si32 {
  // expected-error@+1 {{the failure edge must construct ConversionError, not core::ArithmeticError}}
  %c, %failed, %nf = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite"]}> : (f64) -> (si32, i1, i1)
  cf.cond_br %failed, ^failure, ^success
^failure:
  %e = "sec.union.construct"() <{fieldNames = [], payloadActions = [], variant = 0 : i32}> : () -> !sec.union<identity = "core::ArithmeticError", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "NonFinite", kind = "empty", fields = []>]>
  %z = "sec.const.int"() <{value = "0"}> : () -> si32
  return %z : si32
^success:
  return %c : si32
}
