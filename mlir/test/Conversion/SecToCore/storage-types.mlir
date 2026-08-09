// RUN: sec-mlir-opt %s --sec-lower-trivial-core | FileCheck %s

module {
  func.func @types(%b: i1, %u: ui64, %wide: si256, %f: f64) {
    %bs = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "b", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<i1>
    %us = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "u", sec.storage_class = "local-automatic", sec.storage_id = 2 : i32} : () -> !sec.storage<ui64>
    %fs = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "f", sec.storage_class = "local-automatic", sec.storage_id = 3 : i32} : () -> !sec.storage<f64>
    %ws = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "wide", sec.storage_class = "local-automatic", sec.storage_id = 4 : i32} : () -> !sec.storage<si256>
    "sec.storage.init"(%bs, %b) : (!sec.storage<i1>, i1) -> ()
    "sec.storage.init"(%us, %u) : (!sec.storage<ui64>, ui64) -> ()
    "sec.storage.init"(%fs, %f) : (!sec.storage<f64>, f64) -> ()
    "sec.storage.init"(%ws, %wide) : (!sec.storage<si256>, si256) -> ()
    return
  }
}

// CHECK: "sec.storage.declare"() {{.*}} : () -> !sec.storage<i1>
// CHECK: memref.alloca() {{.*}} : memref<ui64>
// CHECK: memref.alloca() {{.*}} : memref<f64>
// CHECK: memref.alloca() {{.*}} : memref<si256>
// CHECK: "sec.storage.init"({{.*}}) : (!sec.storage<i1>, i1) -> ()
// CHECK: memref.store {{.*}} : memref<ui64>
// CHECK: memref.store {{.*}} : memref<f64>
// CHECK: memref.store {{.*}} : memref<si256>
