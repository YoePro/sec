// RUN: sec-mlir-opt %s --sec-lower-trivial-core --mlir-print-debuginfo | FileCheck %s

module attributes {sec.dialect_version = 2 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "bool", sec.source_files = []} {
  func.func @bool_value() -> i1 attributes {sec.extern = false} {
    %value = "sec.const.bool"() <{value = true}> : () -> i1 loc("bool.sec":2:9)
    return %value : i1
  }
}

// CHECK-LABEL: func.func @bool_value
// CHECK: %[[VALUE:.*]] = arith.constant true loc(#[[BOOL_LOC:loc[0-9]+]])
// CHECK: return %[[VALUE]] : i1
// CHECK-NOT: sec.const.bool
// CHECK: #[[BOOL_LOC]] = loc("bool.sec":2:9)
