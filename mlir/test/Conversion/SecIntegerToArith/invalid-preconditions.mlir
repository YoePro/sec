// RUN: not sec-mlir-opt --split-input-file --sec-lower-checked-integers %s 2>&1 | FileCheck %s

// CHECK: target-sized !sec.int/!sec.uint must be resolved before checked integer lowering
module {
  func.func @unresolved(%left: !sec.int, %right: !sec.int) -> !sec.int {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (!sec.int, !sec.int) -> (!sec.int, i1)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    "sec.fail.arithmetic"() <{category = "overflow"}> {sec.operator = "+"} : () -> ()
  ^success:
    return %result : !sec.int
  }
}

// -----

// CHECK: target-sized !sec.int/!sec.uint must be resolved before checked integer lowering
module {
  func.func private @unresolved_signature(!sec.uint) -> !sec.uint
}

// -----

// CHECK: must be immediately followed by cf.cond_br on failed result
module {
  func.func @malformed_guard(%left: si32, %right: si32) -> si32 {
    %result, %failed = "sec.int.binary_checked"(%left, %right) <{kind = "add"}> : (si32, si32) -> (si32, i1)
    return %result : si32
  }
}

// -----

// CHECK: extern integer argument 0 requires sec.scalar_kind
module {
  func.func private @missing_provenance(si32) attributes {sec.extern = true}
}

// -----

// CHECK: extern integer result 0 requires sec.scalar_kind
module {
  func.func private @missing_result_provenance() -> si128 attributes {sec.extern = true}
}

// -----

// CHECK: extern integer argument 0 has sec.scalar_kind 'uint32' incompatible with 'si32'
module {
  func.func private @wrong_argument_provenance(
      si32 {sec.scalar_kind = "uint32"}) attributes {sec.extern = true}
}

// -----

// CHECK: Sec-origin integer storage requires sec.scalar_kind
module {
  func.func @missing_storage_provenance() {
    %slot = memref.alloca() {sec.storage_id = 1 : i32} : memref<ui32>
    return
  }
}

// -----

// CHECK: Sec-origin integer storage has sec.scalar_kind 'int32' incompatible with 'ui32'
module {
  func.func @wrong_storage_provenance() {
    %slot = memref.alloca() {sec.scalar_kind = "int32", sec.storage_id = 1 : i32} : memref<ui32>
    return
  }
}
