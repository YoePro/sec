// RUN: sec-mlir-opt %s --sec-lower-trivial-core | FileCheck %s

module {
  func.func private @foreign(!sec.int) -> !sec.int attributes {sec.extern = true}
  func.func @mixed(%value: !sec.int) -> !sec.int attributes {sec.extern = false} {
    %integer = "sec.const.int"() <{value = "4"}> : () -> !sec.int
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!sec.int>
    "sec.storage.init"(%slot, %integer) : (!sec.storage<!sec.int>, !sec.int) -> ()
    %result = "sec.call.foreign"(%value) <{callee = @foreign}> {sec.argument_actions = ["copy-trivial"]} : (!sec.int) -> !sec.int
    return %result : !sec.int
  }
  func.func @signed_i1_remains() {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "flag", sec.storage_class = "local-automatic", sec.storage_id = 2 : i32} : () -> !sec.storage<si1>
    return
  }
}

// CHECK: "sec.const.int"
// CHECK: "sec.storage.declare"
// CHECK: "sec.storage.init"
// CHECK: "sec.call.foreign"
// CHECK: "sec.storage.declare"() {{.*}} : () -> !sec.storage<si1>
// CHECK-NOT: memref.alloca
// CHECK-NOT: func.call
