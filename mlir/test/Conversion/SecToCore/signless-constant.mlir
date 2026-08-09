// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout | FileCheck %s

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @signless() -> i128 {
    %value = "sec.const.int"() <{value = "42"}> : () -> i128
    return %value : i128
  }
}

// CHECK: arith.constant 42 : i128
// CHECK-NOT: sec.const.int
