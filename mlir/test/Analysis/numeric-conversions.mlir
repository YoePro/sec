// RUN: sec-mlir-opt %s --sec-verify-numeric-conversions | FileCheck %s

// A checked float-to-integer conversion whose candidate is used only on the
// success edge and whose failure edge tests NonFinite, then OutOfRange, and
// maps each reason to its own ConversionError variant (P20 §§ 41, 56, 81).
module {
  func.func @checked(%x: f64) -> si32 attributes {sec.function_id = "main::Checked(float64)", sec.source_name = "Checked", sec.extern = false, sec.unsafe = false} {
    %c, %failed, %nf, %range = "sec.numeric.convert_checked"(%x) <{failures = ["NonFinite", "OutOfRange"]}> : (f64) -> (si32, i1, i1, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    cf.cond_br %nf, ^nonfinite, ^range
  ^nonfinite:
    %e0 = "sec.union.construct"() <{fieldNames = [], payloadActions = [], variant = 1 : i32}> : () -> !sec.union<identity = "core::ConversionError", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "OutOfRange", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "NonFinite", kind = "empty", fields = []>]>
    %z0 = "sec.const.int"() <{value = "0"}> : () -> si32
    return %z0 : si32
  ^range:
    %e1 = "sec.union.construct"() <{fieldNames = [], payloadActions = [], variant = 0 : i32}> : () -> !sec.union<identity = "core::ConversionError", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "OutOfRange", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "NonFinite", kind = "empty", fields = []>]>
    %z1 = "sec.const.int"() <{value = "0"}> : () -> si32
    return %z1 : si32
  ^success:
    return %c : si32
  }
  func.func @exact(%x: f32) -> f64 attributes {sec.function_id = "main::Exact(float32)", sec.source_name = "Exact", sec.extern = false, sec.unsafe = false} {
    %y = "sec.numeric.convert_exact"(%x) {sec.conversion_proof = {basis = "source-domain", plan_pointer_width = 0 : i32}} : (f32) -> f64
    return %y : f64
  }
}

// CHECK-LABEL: func.func @checked
// CHECK: sec.numeric.convert_checked
// CHECK-LABEL: func.func @exact
