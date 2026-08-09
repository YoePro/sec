// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout | FileCheck %s

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @resolve(%i: !sec.int, %u: !sec.uint, %wide: si128, %decimal: !sec.decimal128) -> !sec.uint {
    return %u : !sec.uint
  }
}

// CHECK: func.func @resolve(%arg0: si64, %arg1: ui64, %arg2: si128, %arg3: !sec.decimal128) -> ui64
// CHECK-NOT: !sec.int
// CHECK-NOT: !sec.uint
// CHECK-NOT: llvm.
