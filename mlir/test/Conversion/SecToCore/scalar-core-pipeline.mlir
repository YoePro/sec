// RUN: sec-mlir-opt %s --sec-lower-scalar-core --sec-lower-scalar-core | FileCheck %s

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @callee(%value: !sec.int) -> !sec.int attributes {sec.extern = false} {
    return %value : !sec.int
  }
  func.func @pipeline(%input: !sec.int) -> !sec.int attributes {sec.extern = false} {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!sec.int>
    "sec.storage.init"(%slot, %input) : (!sec.storage<!sec.int>, !sec.int) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<!sec.int>) -> !sec.int
    %result = "sec.call.direct"(%loaded) <{callee = @callee}> {sec.argument_actions = ["copy-trivial"]} : (!sec.int) -> !sec.int
    return %result : !sec.int
  }
}

// CHECK: func.func @pipeline(%arg0: si64) -> si64
// CHECK: memref.alloca() {{.*}} : memref<si64>
// CHECK: memref.store {{.*}} : memref<si64>
// CHECK: memref.load {{.*}} : memref<si64>
// CHECK: call @callee(%{{.*}}) : (si64) -> si64
// CHECK-NOT: sec.storage
// CHECK-NOT: sec.call.direct
// CHECK-NOT: unrealized_conversion_cast
// CHECK-NOT: llvm.
