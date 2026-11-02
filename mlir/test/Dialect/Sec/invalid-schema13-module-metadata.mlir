// RUN: not sec-mlir-opt %s -o /dev/null 2>&1 | FileCheck %s

module attributes {sec.dialect_version = 13 : i32} {
}

// CHECK: schema 13 requires sec.semantic_ir_version
