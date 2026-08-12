// RUN: split-file %s %t
// RUN: not sec-mlir-opt --sec-verify-union-guards %t/false-path.mlir -o /dev/null 2>&1 | FileCheck %s --check-prefix=FALSE-PATH
// RUN: not sec-mlir-opt --sec-verify-union-guards %t/wrong-variant.mlir -o /dev/null 2>&1 | FileCheck %s --check-prefix=WRONG-VARIANT
// RUN: not sec-mlir-opt --sec-verify-union-guards %t/different-value.mlir -o /dev/null 2>&1 | FileCheck %s --check-prefix=DIFFERENT-VALUE
// RUN: not sec-mlir-opt %t/wrong-kind.mlir -o /dev/null 2>&1 | FileCheck %s --check-prefix=WRONG-KIND
// RUN: not sec-mlir-opt %t/unknown-field.mlir -o /dev/null 2>&1 | FileCheck %s --check-prefix=UNKNOWN-FIELD

// FALSE-PATH: error: 'sec.union.unwrap_payload' op must be in the true successor of a matching union.is_variant guard on the same SSA value
// WRONG-VARIANT: error: 'sec.union.unwrap_payload' op must be in the true successor of a matching union.is_variant guard on the same SSA value
// DIFFERENT-VALUE: error: 'sec.union.unwrap_payload' op must be in the true successor of a matching union.is_variant guard on the same SSA value
// WRONG-KIND: error: 'sec.union.unwrap_payload' op projection requires matching single-payload variant
// UNKNOWN-FIELD: error: 'sec.union.unwrap_field' op field does not exist with the projected result type

//--- false-path.mlir
module {
  func.func @false_path(%value: !sec.union<identity = "main::Option", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> si32 {
    %is_some = "sec.union.is_variant"(%value) <{variant = 0 : i32}> : (!sec.union<identity = "main::Option", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> i1
    cf.cond_br %is_some, ^none, ^some
  ^none:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    return %zero : si32
  ^some:
    %payload = "sec.union.unwrap_payload"(%value) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "main::Option", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> si32
    return %payload : si32
  }
}

//--- wrong-variant.mlir
module {
  func.func @wrong_variant(%value: !sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "First", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "Second", kind = "single", payload = si32, fields = []>]>) -> si32 {
    %is_first = "sec.union.is_variant"(%value) <{variant = 0 : i32}> : (!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "First", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "Second", kind = "single", payload = si32, fields = []>]>) -> i1
    cf.cond_br %is_first, ^first, ^other
  ^first:
    %payload = "sec.union.unwrap_payload"(%value) <{payloadAction = "copy-trivial", variant = 1 : i32}> : (!sec.union<identity = "main::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "First", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "Second", kind = "single", payload = si32, fields = []>]>) -> si32
    return %payload : si32
  ^other:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    return %zero : si32
  }
}

//--- different-value.mlir
module {
  func.func @different_value(
      %tested: !sec.union<identity = "main::Option", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>,
      %projected: !sec.union<identity = "main::Option", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> si32 {
    %is_some = "sec.union.is_variant"(%tested) <{variant = 0 : i32}> : (!sec.union<identity = "main::Option", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> i1
    cf.cond_br %is_some, ^some, ^none
  ^some:
    %payload = "sec.union.unwrap_payload"(%projected) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "main::Option", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = si32, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> si32
    return %payload : si32
  ^none:
    %zero = "sec.const.int"() <{value = "0"}> : () -> si32
    return %zero : si32
  }
}

//--- wrong-kind.mlir
module {
  func.func @wrong_kind(%value: !sec.union<identity = "main::Shape", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = si32>]>]>) -> si32 {
    %payload = "sec.union.unwrap_payload"(%value) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "main::Shape", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = si32>]>]>) -> si32
    return %payload : si32
  }
}

//--- unknown-field.mlir
module {
  func.func @unknown_field(%value: !sec.union<identity = "main::Shape", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = si32>]>]>) -> si32 {
    %field = "sec.union.unwrap_field"(%value) <{field = "y", payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "main::Shape", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = si32>]>]>) -> si32
    return %field : si32
  }
}
