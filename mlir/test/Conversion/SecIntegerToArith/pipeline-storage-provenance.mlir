// RUN: sec-mlir-opt --sec-lower-integer-core --verify-each %s | FileCheck %s

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @integer_storage(%value: !sec.int {sec.scalar_kind = "int"})
      -> (!sec.int {sec.scalar_kind = "int"}) attributes {sec.extern = false} {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "int", sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!sec.int>
    "sec.storage.init"(%slot, %value) : (!sec.storage<!sec.int>, !sec.int) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<!sec.int>) -> !sec.int
    return %loaded : !sec.int
  }

  func.func @rune_storage(%value: !sec.rune {sec.scalar_kind = "rune"})
      -> (!sec.rune {sec.scalar_kind = "rune"}) attributes {sec.extern = false} {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "rune", sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 2 : i32} : () -> !sec.storage<!sec.rune>
    "sec.storage.init"(%slot, %value) : (!sec.storage<!sec.rune>, !sec.rune) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<!sec.rune>) -> !sec.rune
    return %loaded : !sec.rune
  }
}

// CHECK-LABEL: func.func @integer_storage(%{{.*}}: i64 {sec.scalar_kind = "int"}) -> (i64 {sec.scalar_kind = "int"})
// CHECK: %[[INT_SLOT:.*]] = memref.alloca() {{.*}}sec.scalar_kind = "int"{{.*}} : memref<i64>
// CHECK: memref.store {{.*}}, %[[INT_SLOT]][] : memref<i64>
// CHECK: memref.load %[[INT_SLOT]][] : memref<i64>
// CHECK-LABEL: func.func @rune_storage(%{{.*}}: i32 {sec.scalar_kind = "rune"}) -> (i32 {sec.scalar_kind = "rune"})
// CHECK: %[[RUNE_SLOT:.*]] = memref.alloca() {{.*}}sec.scalar_kind = "rune"{{.*}} : memref<i32>
// CHECK: memref.store {{.*}}, %[[RUNE_SLOT]][] : memref<i32>
// CHECK: memref.load %[[RUNE_SLOT]][] : memref<i32>
// CHECK-NOT: !sec.storage
// CHECK-NOT: memref<si
// CHECK-NOT: memref<ui
// CHECK-NOT: unrealized_conversion_cast
// CHECK-NOT: sec.int.
// CHECK-NOT: llvm.
