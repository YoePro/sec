// RUN: sec-mlir-opt --sec-lower-integer-core --verify-each %s | FileCheck %s --implicit-check-not="!sec.named" --implicit-check-not="!sec.storage" --implicit-check-not=unrealized_conversion_cast
// RUN: sec-mlir-opt --sec-lower-integer-core --sec-lower-integer-core --verify-each %s | FileCheck %s --implicit-check-not="!sec.named" --implicit-check-not="!sec.storage" --implicit-check-not=unrealized_conversion_cast
// RUN: sed 's/index, 64/index, 32/' %s | sec-mlir-opt --sec-lower-integer-core --verify-each | FileCheck %s --check-prefix=WIDTH32 --implicit-check-not="!sec.named" --implicit-check-not="!sec.storage"
// Nominal chains are erased only at physical lowering, including local
// storage; carrier provenance and slot identity remain intact.
// Rules: rules/compiler/semantic_ir.md section 7(3)-(5);
// rules/types/types.md section 22.1(1)-(6).

!meters = !sec.named<"main::Meters", !sec.int>
!distance = !sec.named<"main::Distance", !meters>
!count = !sec.named<"main::Count", ui8>
!wide = !sec.named<"main::Wide", ui256>
module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @signed(%initial: !distance) -> !distance {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "int", sec.source_name = "distance", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!distance>
    "sec.storage.init"(%slot, %initial) : (!sec.storage<!distance>, !distance) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<!distance>) -> !distance
    "sec.storage.store"(%slot, %loaded) : (!sec.storage<!distance>, !distance) -> ()
    %result = "sec.storage.load"(%slot) : (!sec.storage<!distance>) -> !distance
    return %result : !distance
  }
  func.func @unsigned(%initial: !count) -> !count {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "uint8", sec.source_name = "count", sec.storage_class = "local-automatic", sec.storage_id = 2 : i32} : () -> !sec.storage<!count>
    "sec.storage.init"(%slot, %initial) : (!sec.storage<!count>, !count) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<!count>) -> !count
    return %loaded : !count
  }
  func.func @wide(%initial: !wide) -> !wide {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "uint256", sec.source_name = "wide", sec.storage_class = "local-automatic", sec.storage_id = 3 : i32} : () -> !sec.storage<!wide>
    "sec.storage.init"(%slot, %initial) : (!sec.storage<!wide>, !wide) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<!wide>) -> !wide
    return %loaded : !wide
  }
}
// CHECK-LABEL: func.func @signed(
// CHECK-SAME: %[[INITIAL:.*]]: i64) -> i64
// CHECK: %[[SLOT:.*]] = memref.alloca() {{.*}}sec.scalar_kind = "int"{{.*}}sec.storage_id = 1 : i32{{.*}} : memref<i64>
// CHECK: memref.store %[[INITIAL]], %[[SLOT]][] : memref<i64>
// CHECK: %[[LOADED:.*]] = memref.load %[[SLOT]][] : memref<i64>
// CHECK: memref.store %[[LOADED]], %[[SLOT]][] : memref<i64>
// CHECK: memref.load %[[SLOT]][] : memref<i64>
// CHECK-LABEL: func.func @unsigned(%{{.*}}: i8) -> i8
// CHECK: memref.alloca() {{.*}}sec.scalar_kind = "uint8"{{.*}} : memref<i8>
// CHECK: memref.store {{.*}} : memref<i8>
// CHECK: memref.load {{.*}} : memref<i8>
// CHECK-LABEL: func.func @wide(%{{.*}}: i256) -> i256
// CHECK: memref.alloca() {{.*}}sec.scalar_kind = "uint256"{{.*}} : memref<i256>
// CHECK: memref.store {{.*}} : memref<i256>
// CHECK: memref.load {{.*}} : memref<i256>
// CHECK-NOT: !sec.named
// CHECK-NOT: !sec.storage
// CHECK-NOT: unrealized_conversion_cast

// WIDTH32: func.func @signed(%{{.*}}: i32) -> i32
// WIDTH32: memref<i32>
// WIDTH32: func.func @unsigned(%{{.*}}: i8) -> i8
// WIDTH32: func.func @wide(%{{.*}}: i256) -> i256
