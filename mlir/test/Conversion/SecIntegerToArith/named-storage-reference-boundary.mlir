// RUN: sec-mlir-opt --sec-lower-integer-core --verify-each %s | FileCheck %s
// Address-taken nominal storage is still a semantic storage root at the P15
// boundary; physical local-storage support must not erase that root.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md section 67;
// rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md section 4(6).
!meters = !sec.named<"main::Meters", !sec.int>
module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @retained(%initial: !meters) -> !meters {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.address_taken = true, sec.scalar_kind = "int", sec.source_name = "meters", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!meters>
    "sec.storage.init"(%slot, %initial) : (!sec.storage<!meters>, !meters) -> ()
    %result = "sec.storage.load"(%slot) : (!sec.storage<!meters>) -> !meters
    return %result : !meters
  }
}
// CHECK-LABEL: func.func @retained
// CHECK: "sec.storage.declare"() {{.*}}sec.address_taken = true{{.*}} : () -> !sec.storage<!sec.named<"main::Meters", si64>>
// CHECK: "sec.storage.init"
// CHECK: "sec.storage.load"
// CHECK-NOT: memref.alloca
