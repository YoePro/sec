// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  "unknown.op"() : () -> ()
}

// CHECK: operation being parsed with an unregistered dialect
