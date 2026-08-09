// RUN: not sec-mlir-opt %s --sec-resolve-scalar-layout 2>&1 | FileCheck %s

module {
  func.func @missing(%value: !sec.int) -> !sec.int {
    return %value : !sec.int
  }
}

// CHECK: scalar layout resolution requires explicit dlti.dl_spec
