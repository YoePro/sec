// RUN: sec-mlir-opt --sec-lower-scalar-core %s | FileCheck %s --check-prefix=SCALAR
// RUN: sec-mlir-opt --sec-lower-scalar-core --sec-lower-checked-integers %s | FileCheck %s --check-prefix=CHECKED

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 9 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema9-struct-scalar-layout-64",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @nested(
      %source: !sec.struct<identity = "main::Outer", typeArguments = [!sec.uint], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>, tags = []>, #sec.struct_field<ordinal = 1, name = "count", type = !sec.uint, tags = []>]>)
      -> !sec.struct<identity = "main::Outer", typeArguments = [!sec.uint], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>, tags = []>, #sec.struct_field<ordinal = 1, name = "count", type = !sec.uint, tags = []>]> {
    return %source : !sec.struct<identity = "main::Outer", typeArguments = [!sec.uint], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>, tags = []>, #sec.struct_field<ordinal = 1, name = "count", type = !sec.uint, tags = []>]>
  }
}

// SCALAR: !sec.struct<identity = "main::Outer", typeArguments = [ui64]
// SCALAR-SAME: #sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [si64]
// SCALAR-SAME: #sec.struct_field<ordinal = 0, name = "value", type = si64, tags = [#sec.struct_tag<key = "unit", value = "ticks">]
// SCALAR-SAME: #sec.struct_field<ordinal = 1, name = "count", type = ui64
// SCALAR-NOT: unrealized_conversion_cast
// CHECKED: !sec.struct<identity = "main::Outer", typeArguments = [ui64]
// CHECKED-SAME: typeArguments = [si64]
// CHECKED-SAME: type = si64
// CHECKED-SAME: type = ui64
// CHECKED-NOT: typeArguments = [i64]
// CHECKED-NOT: type = i64
