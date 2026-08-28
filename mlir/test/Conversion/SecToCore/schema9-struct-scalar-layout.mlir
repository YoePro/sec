// RUN: sec-mlir-opt --sec-lower-scalar-core %s | FileCheck %s
// RUN: sec-mlir-opt --sec-lower-trivial-core %s | FileCheck %s --check-prefix=TRIVIAL
// RUN: sec-mlir-opt --sec-lower-scalar-core --sec-lower-checked-integers %s | FileCheck %s --check-prefix=SIGNED

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>,
  sec.dialect_version = 9 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema9-struct-scalar-layout",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "armv7",
  sec.target_triple = "armv7-unknown-linux-gnueabihf",
  sec.target_abi = "eabihf",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @keep_struct(%source: !sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>) -> !sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]> {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "source", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>>
    "sec.storage.init"(%storage, %source) : (!sec.storage<!sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>>, !sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>) -> ()
    %loaded = "sec.storage.load"(%storage) : (!sec.storage<!sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>>) -> !sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>
    return %loaded : !sec.struct<identity = "main::TargetPair", typeArguments = [!sec.int], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = [#sec.struct_tag<key = "unit", value = "ticks">]>]>
  }

  func.func @nested(%source: !sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]>) -> !sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]> {
    %inner = "sec.struct.extract"(%source) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]>) -> !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>
    %replaced = "sec.struct.replace_field"(%source, %inner) <{field = 0 : i32}> : (!sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]>, !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>) -> !sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]>
    return %replaced : !sec.struct<identity = "main::Outer", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "inner", type = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = !sec.int, tags = []>]>, tags = []>]>
  }
}

// CHECK: !sec.struct<identity = "main::TargetPair", typeArguments = [si32]
// CHECK-SAME: #sec.struct_field<ordinal = 0, name = "value", type = si32
// CHECK-SAME: #sec.struct_tag<key = "unit", value = "ticks">
// CHECK: !sec.storage<!sec.struct<identity = "main::TargetPair", typeArguments = [si32]
// CHECK: !sec.struct<identity = "main::Outer"
// CHECK-SAME: !sec.struct<identity = "main::Inner"
// CHECK-SAME: type = si32
// CHECK: "sec.struct.extract"({{.*}}) {{.*}} : (!sec.struct<identity = "main::Outer"
// CHECK: "sec.struct.replace_field"({{.*}}) {{.*}} : (!sec.struct<identity = "main::Outer"
// CHECK-NOT: unrealized_conversion_cast
// TRIVIAL: "sec.storage.declare"
// TRIVIAL-SAME: !sec.storage<!sec.struct<identity = "main::TargetPair"
// TRIVIAL: "sec.storage.init"
// TRIVIAL: "sec.storage.load"
// TRIVIAL: "sec.struct.extract"
// TRIVIAL: "sec.struct.replace_field"
// TRIVIAL-NOT: memref<!sec.struct
// SIGNED: !sec.struct<identity = "main::TargetPair", typeArguments = [si32]
// SIGNED: "sec.struct.extract"({{.*}}) {{.*}} : (!sec.struct<identity = "main::Outer"
// SIGNED-SAME: type = si32
// SIGNED: "sec.struct.replace_field"({{.*}}) {{.*}} : (!sec.struct<identity = "main::Outer"
// SIGNED-NOT: typeArguments = [i32]
