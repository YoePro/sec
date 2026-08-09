// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module attributes {sec.dialect_version = 2 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "types", sec.source_files = []} {
  func.func private @scalars(%a: !sec.int, %b: !sec.uint, %c: !sec.float,
                    %d: !sec.char, %e: !sec.rune, %f: !sec.string,
                    %g: !sec.decimal, %h: !sec.never,
                    %i: !sec.storage<!sec.int>)
}

// CHECK: !sec.int
// CHECK-SAME: !sec.uint
// CHECK-SAME: !sec.float
// CHECK-SAME: !sec.char
// CHECK-SAME: !sec.rune
// CHECK-SAME: !sec.string
// CHECK-SAME: !sec.decimal
// CHECK-SAME: !sec.never
// CHECK-SAME: !sec.storage<!sec.int>
