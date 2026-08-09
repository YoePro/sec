// RUN: not sec-mlir-opt --split-input-file --sec-verify-checked-integer-guards %s 2>&1 | FileCheck %s

// CHECK: op must be immediately followed by cf.cond_br on failed result
module {
  func.func @unused_failure(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    return %result : si32
  }
}

// -----

// CHECK: op failed result must have exactly one use
module {
  func.func @failure_used_twice(%left: si32, %right: si32) -> i1 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %failed : i1
  }
}

// -----

// CHECK: op must be immediately followed by cf.cond_br on failed result
module {
  func.func @non_branch_use(%left: si32, %right: si32) -> i1 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    %copy = arith.andi %failed, %failed : i1
    return %copy : i1
  }
}

// -----

// CHECK: op true successor must end in sec.fail.arithmetic
module {
  func.func @reversed_targets(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^success, ^failure
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : si32
  }
}

// -----

// CHECK: op must be immediately followed by cf.cond_br on failed result
module {
  func.func @intervening_operation(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    %copy = "sec.int.unary_plus"(%result) : (si32) -> si32
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %copy : si32
  }
}

// -----

// CHECK: op true successor must be a dedicated one-operation failure block
module {
  func.func @extra_failure_operation(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    %zero = arith.constant 0 : i32
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : si32
  }
}

// -----

// CHECK: op true successor must end in sec.fail.arithmetic
module {
  func.func @missing_failure_terminator(%left: si32, %right: si32) {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    return
  ^success:
    return
  }
}

// -----

// CHECK: op failure category does not match checked operation
module {
  func.func @wrong_category(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "divide"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "/"} : () -> ()
  ^success:
    return %result : si32
  }
}

// -----

// CHECK: op sec.operator does not match checked operation
module {
  func.func @wrong_operator(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "multiply"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : si32
  }
}

// -----

// CHECK: op checked result must not be used in failure block
module {
  func.func @result_used_on_failure(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    %copy = "sec.int.unary_plus"(%result) : (si32) -> si32
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : si32
  }
}
