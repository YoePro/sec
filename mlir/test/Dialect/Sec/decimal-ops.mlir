// RUN: sec-mlir-opt %s | FileCheck %s

// P21 decimal operators (sec_mlir_dialect_v17.md §§ 3, 6-7; MD-066/MD-067):
// checked binary and unary forms carry the Sema plan and yield (candidate,
// failed, reason); comparison is total and returns i1.
module {
  func.func @ops(%a: !sec.decimal, %b: !sec.decimal, %w: !sec.decimal128) -> i1 {
    %s, %sf, %sr = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["scale-overflow", "precision-exhausted"], kind = "add", preferred_scale = "max-operand-scale", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
    %q, %qf, %qr = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["division-by-zero", "scale-overflow", "precision-exhausted", "overflow"], kind = "div", preferred_scale = "max-operand-scale", quotient = "nearest-even", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
    %r, %rf, %rr = "sec.decimal.binary_checked"(%a, %b) <{coefficient_bits = 64 : i32, failures = ["division-by-zero", "scale-overflow", "precision-exhausted"], kind = "rem", preferred_scale = "max-operand-scale", remainder = "truncating", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)
    %n, %nf, %nr = "sec.decimal.unary"(%w) <{coefficient_bits = 128 : i32, failures = ["scale-overflow", "precision-exhausted"], kind = "neg", preferred_scale = "operand-scale", scale_policy = "nearest-exact"}> : (!sec.decimal128) -> (!sec.decimal128, i1, !sec.arithmetic_failure_reason)
    %c = "sec.decimal.cmp"(%a, %b) <{predicate = "le"}> : (!sec.decimal, !sec.decimal) -> i1
    return %c : i1
  }
}

// CHECK: "sec.decimal.binary_checked"{{.*}}kind = "add"
// CHECK: "sec.decimal.binary_checked"{{.*}}quotient = "nearest-even"
// CHECK: "sec.decimal.binary_checked"{{.*}}remainder = "truncating"
// CHECK: "sec.decimal.unary"{{.*}}kind = "neg"
// CHECK: "sec.decimal.cmp"{{.*}}predicate = "le"
