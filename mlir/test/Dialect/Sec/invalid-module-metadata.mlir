// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module attributes {sec.dialect_version = 2 : i32, sec.semantic_ir_version = 1 : i32, sec.source_files = []} {
}

// CHECK: schema 2 requires a non-empty sec.module_id string
