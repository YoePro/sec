// RUN: not sec-mlir-opt %s --sec-resolve-scalar-layout 2>&1 | FileCheck %s

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @legacy() {
    %slot = memref.alloca() {sec.mutable = true, sec.source_name = "flag", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : memref<i1>
    return
  }
}

// CHECK: legacy Sec bool storage memref<i1> is invalid; addressable bool storage requires memref<i8>
