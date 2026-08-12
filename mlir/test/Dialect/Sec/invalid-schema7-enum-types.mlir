// RUN: split-file %s %t
// RUN: not sec-mlir-opt %t/signed.mlir 2>&1 | FileCheck %s --check-prefix=SIGNED
// RUN: not sec-mlir-opt %t/wrong-width.mlir 2>&1 | FileCheck %s --check-prefix=WIDTH

//--- signed.mlir
module attributes {sec.dialect = "sec", sec.dialect_version = 7 : i32} {
  func.func @signed(%value: !sec.enum<identity = "main::Bits", underlying = si8, representation = "bit-backed", bitWidth = 8, cases = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">]>) {
    return
  }
}

// SIGNED: bit-backed sec.enum underlying type must be unsigned and match bitWidth

//--- wrong-width.mlir
module attributes {sec.dialect = "sec", sec.dialect_version = 7 : i32} {
  func.func @wrong_width(%value: !sec.enum<identity = "main::Bits", underlying = ui16, representation = "bit-backed", bitWidth = 8, cases = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">]>) {
    return
  }
}

// WIDTH: bit-backed sec.enum underlying type must be unsigned and match bitWidth
