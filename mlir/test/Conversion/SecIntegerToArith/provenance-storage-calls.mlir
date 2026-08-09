// RUN: sec-mlir-opt --sec-lower-checked-integers %s | FileCheck %s

module {
  func.func private @foreign(
      ui32 {sec.scalar_kind = "uint32"})
      -> (si128 {sec.scalar_kind = "int128"})
      attributes {sec.extern = true}

  func.func @storage(%value: si32 {sec.scalar_kind = "int32"})
      -> (si32 {sec.scalar_kind = "int32"})
      attributes {sec.extern = false} {
    %slot = memref.alloca() {sec.mutable = true, sec.scalar_kind = "int32", sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : memref<si32>
    memref.store %value, %slot[] : memref<si32>
    %loaded = memref.load %slot[] : memref<si32>
    return %loaded : si32
  }

  func.func @wide_storage(%value: ui256 {sec.scalar_kind = "uint256"})
      -> (ui256 {sec.scalar_kind = "uint256"})
      attributes {sec.extern = false} {
    %slot = memref.alloca() {sec.mutable = true, sec.scalar_kind = "uint256", sec.source_name = "wide", sec.storage_class = "local-automatic", sec.storage_id = 2 : i32} : memref<ui256>
    memref.store %value, %slot[] : memref<ui256>
    %loaded = memref.load %slot[] : memref<ui256>
    return %loaded : ui256
  }

  func.func @bool_storage(%value: i8 {sec.scalar_kind = "bool"}) -> i8 {
    %slot = memref.alloca() {sec.mutable = true, sec.scalar_kind = "bool", sec.source_name = "flag", sec.storage_class = "local-automatic", sec.storage_id = 3 : i32} : memref<i8>
    memref.store %value, %slot[] : memref<i8>
    %loaded = memref.load %slot[] : memref<i8>
    return %loaded : i8
  }

  func.func @call_foreign(%value: ui32 {sec.scalar_kind = "uint32"})
      -> (si128 {sec.scalar_kind = "int128"})
      attributes {sec.extern = false} {
    %result = "sec.call.foreign"(%value) <{callee = @foreign}> {sec.argument_actions = ["copy-trivial"]} : (ui32) -> si128
    return %result : si128
  }
}

// CHECK-LABEL: func.func private @foreign(i32 {sec.scalar_kind = "uint32"}) -> (i128 {sec.scalar_kind = "int128"})
// CHECK-LABEL: func.func @storage(%{{.*}}: i32 {sec.scalar_kind = "int32"}) -> (i32 {sec.scalar_kind = "int32"})
// CHECK: %[[SLOT:.*]] = memref.alloca() {{.*}}sec.scalar_kind = "int32"{{.*}} : memref<i32>
// CHECK: memref.store {{.*}}, %[[SLOT]][] : memref<i32>
// CHECK: memref.load %[[SLOT]][] : memref<i32>
// CHECK-LABEL: func.func @wide_storage(%{{.*}}: i256 {sec.scalar_kind = "uint256"}) -> (i256 {sec.scalar_kind = "uint256"})
// CHECK: %[[WIDE:.*]] = memref.alloca() {{.*}}sec.scalar_kind = "uint256"{{.*}} : memref<i256>
// CHECK: memref.store {{.*}}, %[[WIDE]][] : memref<i256>
// CHECK: memref.load %[[WIDE]][] : memref<i256>
// CHECK-LABEL: func.func @bool_storage
// CHECK: %[[BOOL:.*]] = memref.alloca() {{.*}}sec.scalar_kind = "bool"{{.*}} : memref<i8>
// CHECK: memref.store {{.*}}, %[[BOOL]][] : memref<i8>
// CHECK: memref.load %[[BOOL]][] : memref<i8>
// CHECK-LABEL: func.func @call_foreign
// CHECK: "sec.call.foreign"
// CHECK-SAME: (i32) -> i128
// CHECK-NOT: func.call @foreign
// CHECK-NOT: memref<si
// CHECK-NOT: memref<ui
