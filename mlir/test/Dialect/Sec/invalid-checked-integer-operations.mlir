// RUN: not sec-mlir-opt --split-input-file %s 2>&1 | FileCheck %s

// CHECK: op result #1 must be 1-bit signless integer
module {
  func.func @failure_not_i1(%value: si32) {
    %result:2 = "sec.int.neg_checked"(%value) : (si32) -> (si32, i8)
    return
  }
}

// -----

// CHECK: op shift kind signedness must match value type
module {
  func.func @signed_shift_on_unsigned(%value: ui32, %count: si8) {
    %result, %failed = "sec.int.shift_checked"(%value, %count) <{kind = "left_signed"}> : (ui32, si8) -> (ui32, i1)
    return
  }
}

// -----

// CHECK: op shift kind signedness must match value type
module {
  func.func @unsigned_shift_on_signed(%value: si32, %count: ui16) {
    %result, %failed = "sec.int.shift_checked"(%value, %count) <{kind = "right_unsigned"}> : (si32, ui16) -> (si32, i1)
    return
  }
}

// -----

// CHECK: op operand must be a builtin Sec integer semantic type
module {
  func.func @named_integer(%value: !sec.named<"main::Count", si32>) {
    %result = "sec.int.unary_plus"(%value) : (!sec.named<"main::Count", si32>) -> !sec.named<"main::Count", si32>
    return
  }
}

// -----

// CHECK: op invalid arithmetic failure category
module {
  func.func @unknown_failure_category() {
    "sec.fail.arithmetic"() <{category = "unknown"}> {sec.operator = "+"} : () -> ()
  }
}

// -----

// CHECK: op must be the last operation in the parent block
module {
  func.func @failure_is_terminator() {
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
    return
  }
}
