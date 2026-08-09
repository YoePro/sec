// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module attributes {sec.dialect_version = 2 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "constants", sec.source_files = []} {
  func.func @constants() attributes {sec.extern = false} {
    %i = "sec.const.int"() <{value = "9223372036854775807"}> : () -> si64
    %u = "sec.const.int"() <{value = "18446744073709551615"}> : () -> ui64
    %b = "sec.const.bool"() <{value = true}> : () -> i1
    %f = "sec.const.float"() <{lexeme = "1.25e2"}> : () -> !sec.float
    %d = "sec.const.decimal"() <{coefficient = "-125", lexeme = "-1.25", scale = 2 : i32}> : () -> !sec.decimal
    %s = "sec.const.string"() <{value = "hej\nvarld"}> : () -> !sec.string
    return
  }
}

// CHECK: "sec.const.int"() <{value = "9223372036854775807"}> : () -> si64
// CHECK: "sec.const.int"() <{value = "18446744073709551615"}> : () -> ui64
// CHECK: "sec.const.bool"() <{value = true}> : () -> i1
// CHECK: "sec.const.float"() <{lexeme = "1.25e2"}> : () -> !sec.float
// CHECK: "sec.const.decimal"() <{coefficient = "-125", lexeme = "-1.25", scale = 2 : i32}> : () -> !sec.decimal
// CHECK: "sec.const.string"() <{value = "hej\0Avarld"}> : () -> !sec.string
