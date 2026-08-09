// RUN: sec-mlir-opt %s --sec-lower-trivial-core --mlir-print-debuginfo | FileCheck %s

module attributes {sec.dialect_version = 2 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "storage", sec.source_files = []} {
  func.func @storage(%input: si32) -> si32 attributes {sec.extern = false} {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "int32", sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 7 : i32} : () -> !sec.storage<si32> loc("storage.sec":3:5)
    "sec.storage.init"(%slot, %input) : (!sec.storage<si32>, si32) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<si32>) -> si32
    "sec.storage.store"(%slot, %loaded) : (!sec.storage<si32>, si32) -> ()
    return %loaded : si32
  }
}

// CHECK-LABEL: func.func @storage
// CHECK: %[[SLOT:.*]] = memref.alloca() {sec.mutable = true, sec.scalar_kind = "int32", sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 7 : i32} : memref<si32> loc(#[[STORAGE_LOC:loc[0-9]+]])
// CHECK: memref.store %arg0, %[[SLOT]][] : memref<si32>
// CHECK: %[[VALUE:.*]] = memref.load %[[SLOT]][] : memref<si32>
// CHECK: memref.store %[[VALUE]], %[[SLOT]][] : memref<si32>
// CHECK-NOT: sec.storage
// CHECK-NOT: memref.dealloc
// CHECK-NOT: unrealized_conversion_cast
// CHECK: #[[STORAGE_LOC]] = loc("storage.sec":3:5)
