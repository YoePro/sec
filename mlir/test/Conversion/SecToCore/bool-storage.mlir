// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout | FileCheck %s

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bool_storage(%input: i1) -> i1 {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "flag", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<i1>
    "sec.storage.init"(%slot, %input) : (!sec.storage<i1>, i1) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<i1>) -> i1
    "sec.storage.store"(%slot, %loaded) : (!sec.storage<i1>, i1) -> ()
    return %loaded : i1
  }
}

// CHECK: %[[SLOT:.*]] = memref.alloca() {{.*}} : memref<i8>
// CHECK: %[[INIT:.*]] = arith.extui %arg0 : i1 to i8
// CHECK: memref.store %[[INIT]], %[[SLOT]][] : memref<i8>
// CHECK: %[[BYTE:.*]] = memref.load %[[SLOT]][] : memref<i8>
// CHECK: %[[BOOL:.*]] = arith.trunci %[[BYTE]] : i8 to i1
// CHECK: %[[STORE:.*]] = arith.extui %[[BOOL]] : i1 to i8
// CHECK: memref.store %[[STORE]], %[[SLOT]][] : memref<i8>
// CHECK: return %[[BOOL]] : i1
// CHECK-NOT: memref<i1>
// CHECK-NOT: unrealized_conversion_cast
