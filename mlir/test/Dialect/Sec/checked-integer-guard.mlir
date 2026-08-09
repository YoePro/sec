// RUN: sec-mlir-opt --sec-verify-checked-integer-guards %s | FileCheck %s

module {
  func.func @checked(%left: si128, %right: si128) -> si128 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si128, si128) -> (si128, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : si128
  }
}

module {
  func.func @nested(%left: si64, %middle: si64, %right: si64) -> si64 {
    %product, %multiply_failed = "sec.int.binary_checked"(%middle, %right) <{kind = "multiply"}> : (si64, si64) -> (si64, i1)
    cf.cond_br %multiply_failed, ^multiply_failure, ^after_multiply
  ^multiply_failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "*"} : () -> ()
  ^after_multiply:
    %sum, %add_failed = "sec.int.binary_checked"(%left, %product) <{kind = "add"}> : (si64, si64) -> (si64, i1)
    cf.cond_br %add_failed, ^add_failure, ^success
  ^add_failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %sum : si64
  }
}

// CHECK: "sec.int.binary_checked"
// CHECK-NEXT: cf.cond_br
// CHECK: "sec.fail.arithmetic"
// CHECK-LABEL: func.func @nested
// CHECK: "sec.int.binary_checked"
// CHECK-NEXT: cf.cond_br
// CHECK: "sec.fail.arithmetic"
// CHECK: "sec.int.binary_checked"
// CHECK-NEXT: cf.cond_br
