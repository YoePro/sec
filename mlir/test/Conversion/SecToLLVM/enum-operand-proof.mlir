// RUN: sec-mlir-opt %s --sec-lower-to-llvm=entry= | FileCheck %s

// A compiler proof may establish exactness for this operand without proving
// every value in its enum fits. Lowering must consume that proof and must not
// substitute its own whole-enum domain test (semantic_ir.md § 124(2–3)).
!Choice = !sec.enum<identity = "main::Choice", underlying = si16, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "Negative", value = "-1000">, #sec.enum_case<ordinal = 1, name = "One", value = "1">]>
module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @one() -> ui8 {
    %v = "sec.enum.constant"() <{caseOrdinal = 1 : i32}> : () -> !Choice
    %out = "sec.enum.to_integer"(%v) {sec.conversion_proof = {basis = "constant", plan_pointer_width = 0 : i32}} : (!Choice) -> ui8
    return %out : ui8
  }
}
// CHECK-LABEL: llvm.func @one() -> i8
// CHECK: llvm.mlir.constant(1 : i8)
// CHECK-NOT: sec.enum
