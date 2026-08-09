// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid(%value: i64) {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "x", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<i32>
    "sec.storage.init"(%slot, %value) : (!sec.storage<i32>, i64) -> ()
    return
  }
}

// CHECK: storage element and value types must match exactly
