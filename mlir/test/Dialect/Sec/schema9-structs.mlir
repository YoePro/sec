// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// RUN: sec-mlir-opt %S/schema8-unreachable.mlir | FileCheck %S/schema8-unreachable.mlir

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 9 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema9-structs",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @empty(%value: !sec.struct<identity = "main::Empty", typeArguments = [], fields = []>) -> !sec.struct<identity = "main::Empty", typeArguments = [], fields = []> {
    %new = "sec.struct.construct"() <{field_actions = [], field_origins = []}> : () -> !sec.struct<identity = "main::Empty", typeArguments = [], fields = []>
    return %new : !sec.struct<identity = "main::Empty", typeArguments = [], fields = []>
  }

  func.func @nested(%value: !sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]>) -> !sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]> {
    return %value : !sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]>
  }

  func.func @values(
      %wide: si128,
      %limit: ui256,
      %source: !sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]>)
      -> !sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]> {
    %first, %second = "sec.struct.spread_fields"(%source) <{actions = ["copy-trivial", "copy-trivial"]}> : (!sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]>) -> (si128, ui256)
    %made = "sec.struct.construct"(%wide, %second) <{field_actions = ["construct-direct", "copy-trivial"], field_origins = ["explicit", "spread"]}> : (si128, ui256) -> !sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]>
    %read = "sec.struct.extract"(%made) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]>) -> si128
    %replaced = "sec.struct.replace_field"(%made, %limit) <{field = 1 : i32}> : (!sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]>, ui256) -> !sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]>
    return %replaced : !sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256], fields = [#sec.struct_field<ordinal = 0, name = "wide", type = si128, tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]>, #sec.struct_field<ordinal = 1, name = "limit", type = ui256, tags = []>]>
  }
}

// CHECK: sec.dialect_version = 9 : i32
// CHECK: !sec.struct<identity = "main::Empty", typeArguments = [], fields = []>
// CHECK: "sec.struct.construct"
// CHECK: !sec.struct<identity = "main::Outer"
// CHECK: !sec.struct<identity = "main::Pair<int128,uint256>", typeArguments = [si128, ui256]
// CHECK: tags = [#sec.struct_tag<key = "wire", value = "signed">, #sec.struct_tag<key = "json", value = "wide_value">]
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
