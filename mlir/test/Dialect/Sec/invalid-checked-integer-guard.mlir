// RUN: not sec-mlir-opt --sec-verify-checked-integer-guards %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%left: si64, %right: si64) -> si64 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "divide"}> : (si64, si64) -> (si64, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "/"} : () -> ()
  ^success:
    return %result : si64
  }
}

// CHECK: failure category does not match checked operation
