// RUN: not sec-mlir-opt %s --sec-lower-trivial-core 2>&1 | FileCheck %s

// Package 3 storage handles are local implementation details. A handle in a
// function result cannot be half-converted by the Package 5 partial lowering.
module {
  func.func @invalid_boundary() -> !sec.storage<si32> {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<si32>
    return %slot : !sec.storage<si32>
  }
}

// CHECK: error: failed to legalize unresolved materialization
