// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// RUN: sec-mlir-opt --sec-verify-union-guards %s -o /dev/null

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 7 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "enum-union",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @enums(%integer: si128) -> i1 {
    %declared = "sec.enum.constant"() <{caseOrdinal = 1 : i32}> : () -> !sec.enum<identity = "main::Status", underlying = si128, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">, #sec.enum_case<ordinal = 1, name = "huge", value = "170141183460469231731687303715884105727">, #sec.enum_case<ordinal = 2, name = "aliasHuge", value = "170141183460469231731687303715884105727">]>
    %converted = "sec.enum.from_integer"(%integer) : (si128) -> !sec.enum<identity = "main::Status", underlying = si128, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">, #sec.enum_case<ordinal = 1, name = "huge", value = "170141183460469231731687303715884105727">, #sec.enum_case<ordinal = 2, name = "aliasHuge", value = "170141183460469231731687303715884105727">]>
    %numeric = "sec.enum.to_integer"(%converted) : (!sec.enum<identity = "main::Status", underlying = si128, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">, #sec.enum_case<ordinal = 1, name = "huge", value = "170141183460469231731687303715884105727">, #sec.enum_case<ordinal = 2, name = "aliasHuge", value = "170141183460469231731687303715884105727">]>) -> si128
    %equal = "sec.enum.cmp"(%declared, %converted) <{predicate = "eq"}> : (!sec.enum<identity = "main::Status", underlying = si128, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">, #sec.enum_case<ordinal = 1, name = "huge", value = "170141183460469231731687303715884105727">, #sec.enum_case<ordinal = 2, name = "aliasHuge", value = "170141183460469231731687303715884105727">]>, !sec.enum<identity = "main::Status", underlying = si128, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">, #sec.enum_case<ordinal = 1, name = "huge", value = "170141183460469231731687303715884105727">, #sec.enum_case<ordinal = 2, name = "aliasHuge", value = "170141183460469231731687303715884105727">]>) -> i1
    return %equal : i1
  }

  func.func @bit_enums(
      %one: !sec.enum<identity = "main::OneBit", underlying = ui1, representation = "bit-backed", bitWidth = 1, cases = [#sec.enum_case<ordinal = 0, name = "off", value = "0">, #sec.enum_case<ordinal = 1, name = "on", value = "1">]>,
      %wide: !sec.enum<identity = "main::WideBits", underlying = ui256, representation = "bit-backed", bitWidth = 256, cases = [#sec.enum_case<ordinal = 0, name = "top", value = "57896044618658097711785492504343953926634992332820282019728792003956564819968">]>) {
    return
  }

  func.func @construct(%value: !sec.int, %x: !sec.int, %y: !sec.int) {
    %none = "sec.union.construct"() <{fieldNames = [], payloadActions = [], variant = 1 : i32}> : () -> !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
    %some = "sec.union.construct"(%value) <{fieldNames = [], payloadActions = ["copy-trivial"], variant = 0 : i32}> : (!sec.int) -> !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
    %point = "sec.union.construct"(%x, %y) <{fieldNames = ["x", "y"], payloadActions = ["copy-trivial", "copy-trivial"], variant = 2 : i32}> : (!sec.int, !sec.int) -> !sec.union<identity = "main::Message", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Quit", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Text", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 2, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = !sec.int>, #sec.union_field<name = "y", type = !sec.int>]>]>
    return
  }

  func.func @guarded(%value: !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> !sec.int {
    %is_some = "sec.union.is_variant"(%value) <{variant = 0 : i32}> : (!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> i1
    cf.cond_br %is_some, ^some, ^none
  ^some:
    %payload = "sec.union.unwrap_payload"(%value) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>) -> !sec.int
    return %payload : !sec.int
  ^none:
    %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    return %zero : !sec.int
  }

  func.func @guarded_field(%value: !sec.union<identity = "main::PointOrNone", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = !sec.int>, #sec.union_field<name = "y", type = !sec.int>]>]>) -> !sec.int {
    %is_point = "sec.union.is_variant"(%value) <{variant = 1 : i32}> : (!sec.union<identity = "main::PointOrNone", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = !sec.int>, #sec.union_field<name = "y", type = !sec.int>]>]>) -> i1
    cf.cond_br %is_point, ^point, ^none
  ^point:
    %x = "sec.union.unwrap_field"(%value) <{field = "x", payloadAction = "copy-trivial", variant = 1 : i32}> : (!sec.union<identity = "main::PointOrNone", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "None", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Point", kind = "fields", fields = [#sec.union_field<name = "x", type = !sec.int>, #sec.union_field<name = "y", type = !sec.int>]>]>) -> !sec.int
    return %x : !sec.int
  ^none:
    %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    return %zero : !sec.int
  }
}

// CHECK: sec.dialect_version = 7 : i32
// CHECK: !sec.enum<identity = "main::Status"
// CHECK: #sec.enum_case<ordinal = 2, name = "aliasHuge"
// CHECK: "sec.enum.from_integer"
// CHECK: "sec.enum.to_integer"
// CHECK: "sec.enum.cmp"
// CHECK: !sec.union<identity = "main::Option<int>"
// CHECK: "sec.union.construct"
// CHECK: "sec.union.is_variant"
// CHECK: "sec.union.unwrap_payload"
// CHECK: "sec.union.unwrap_field"
