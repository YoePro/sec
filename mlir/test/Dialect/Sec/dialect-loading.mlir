// RUN: sec-mlir-opt %s | FileCheck %s

module attributes {sec.dialect_version = 1 : i32} {
}

// CHECK: module attributes {sec.dialect_version = 1 : i32}
