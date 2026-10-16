// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// Struct field and union/Result payload Places keep authority and denote the
// exact field or payload type (sec_mlir_dialect_v11.md sections 10, 13).
module {
  // CHECK-LABEL: func.func @field
  func.func @field(%pair: !sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "rw">) -> !sec.place<!sec.int, "rw"> {
    // CHECK: sec.place.field
    %right = "sec.place.field"(%pair) <{field = 1 : i32}> : (!sec.place<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !sec.int, tags = []>]>, "rw">) -> !sec.place<!sec.int, "rw">
    return %right : !sec.place<!sec.int, "rw">
  }
  // CHECK-LABEL: func.func @payloads
  func.func @payloads(%option: !sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">, %result: !sec.place<!sec.result<!sec.int, !sec.string>, "ro">) {
    // CHECK: sec.place.union_payload
    %some = "sec.place.union_payload"(%option) <{variant = 0 : i32}> : (!sec.place<!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>, "ro">) -> !sec.place<!sec.int, "ro">
    %err = "sec.place.union_payload"(%result) <{variant = 1 : i32}> : (!sec.place<!sec.result<!sec.int, !sec.string>, "ro">) -> !sec.place<!sec.string, "ro">
    return
  }
}
