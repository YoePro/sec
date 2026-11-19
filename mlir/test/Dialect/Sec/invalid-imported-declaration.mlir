// RUN: not sec-mlir-opt --split-input-file %s 2>&1 | FileCheck %s

// An imported Sec declaration cannot carry a body.
module {
  func.func @sec_sym_twice(%value: i64) -> i64 attributes {sec.function_id = "util::Twice(int)", sec.source_name = "util.Twice", sec.extern = false, sec.unsafe = false, sec.imported = true} {
    return %value : i64
  }
}

// CHECK: an imported Sec declaration has no body and is not extern

// -----

// An imported Sec declaration is never a foreign function.
module {
  func.func private @sec_sym_twice(i64) -> i64 attributes {sec.function_id = "util::Twice(int)", sec.source_name = "util.Twice", sec.extern = true, sec.unsafe = false, sec.imported = true}
}

// CHECK: an imported Sec declaration has no body and is not extern
