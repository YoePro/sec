// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module {
  func.func @storage() -> si32 attributes {sec.extern = false} {
    %value = "sec.const.int"() <{value = "7"}> : () -> si32
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "value", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<si32>
    "sec.storage.init"(%slot, %value) : (!sec.storage<si32>, si32) -> ()
    %loaded = "sec.storage.load"(%slot) : (!sec.storage<si32>) -> si32
    "sec.storage.store"(%slot, %loaded) : (!sec.storage<si32>, si32) -> ()
    return %loaded : si32
  }
}

// CHECK: "sec.storage.declare"
// CHECK: "sec.storage.init"
// CHECK: "sec.storage.load"
// CHECK: "sec.storage.store"
