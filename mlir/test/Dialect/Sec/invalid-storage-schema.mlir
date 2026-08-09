// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

module {
  func.func @invalid() {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "x", sec.storage_class = "local-automatic", sec.storage_id = 0 : i32} : () -> !sec.storage<i32>
    return
  }
}

// CHECK: sec.storage_id must be a positive i32 integer
