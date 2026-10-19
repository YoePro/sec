// RUN: not sec-mlir-opt %s -o /dev/null 2>&1 | FileCheck %s

module attributes {sec.dialect_version = 12 : i32} {
}

// CHECK: schema 12 requires sec.semantic_ir_version = 1 : i32
