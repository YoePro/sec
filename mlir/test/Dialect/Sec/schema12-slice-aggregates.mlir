// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md §§114,140.
// Shared aggregate fields copy holders only; referent copyability is independent.
!direct = !sec.slice<si32>
!named = !sec.named<"main::View", !sec.slice<si32>>
!distinct = !sec.distinct<"main::View", !sec.slice<si32>>
!array = !sec.array<!sec.slice<si32>, "2">
!empty_array = !sec.array<!sec.slice<si32>, "0">
!nested = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !sec.slice<si32>, tags = []>]>
!result = !sec.result<!sec.slice<si32>, si32>
!shared_mutable_elements = !sec.slice<!sec.slice_mut<si32>>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @direct(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !direct, tags = []>]>, %field: !direct) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!direct) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !direct, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !direct, tags = []>]>) -> !direct
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !direct, tags = []>]>) -> !direct
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !direct, tags = []>]>, !direct) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !direct, tags = []>]>
    return
  }
  func.func @named(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !named, tags = []>]>, %field: !named) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!named) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !named, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !named, tags = []>]>) -> !named
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !named, tags = []>]>) -> !named
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !named, tags = []>]>, !named) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !named, tags = []>]>
    return
  }
  func.func @distinct(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !distinct, tags = []>]>, %field: !distinct) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!distinct) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !distinct, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !distinct, tags = []>]>) -> !distinct
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !distinct, tags = []>]>) -> !distinct
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !distinct, tags = []>]>, !distinct) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !distinct, tags = []>]>
    return
  }
  func.func @array(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !array, tags = []>]>, %field: !array) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!array) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !array, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !array, tags = []>]>) -> !array
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !array, tags = []>]>) -> !array
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !array, tags = []>]>, !array) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !array, tags = []>]>
    return
  }
  func.func @empty_array(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !empty_array, tags = []>]>, %field: !empty_array) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!empty_array) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !empty_array, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !empty_array, tags = []>]>) -> !empty_array
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !empty_array, tags = []>]>) -> !empty_array
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !empty_array, tags = []>]>, !empty_array) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !empty_array, tags = []>]>
    return
  }
  func.func @nested(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !nested, tags = []>]>, %field: !nested) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!nested) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !nested, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !nested, tags = []>]>) -> !nested
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !nested, tags = []>]>) -> !nested
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !nested, tags = []>]>, !nested) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !nested, tags = []>]>
    return
  }
  func.func @result(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !result, tags = []>]>, %field: !result) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!result) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !result, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !result, tags = []>]>) -> !result
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !result, tags = []>]>) -> !result
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !result, tags = []>]>, !result) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !result, tags = []>]>
    return
  }
  func.func @shared_mutable_elements(%holder: !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !shared_mutable_elements, tags = []>]>, %field: !shared_mutable_elements) {
    %construct = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!shared_mutable_elements) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !shared_mutable_elements, tags = []>]>
    %spread = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !shared_mutable_elements, tags = []>]>) -> !shared_mutable_elements
    %extract = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !shared_mutable_elements, tags = []>]>) -> !shared_mutable_elements
    %replace = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !shared_mutable_elements, tags = []>]>, !shared_mutable_elements) -> !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !shared_mutable_elements, tags = []>]>
    return
  }
}
// CHECK-LABEL: func.func @direct(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
// CHECK-LABEL: func.func @named(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
// CHECK-LABEL: func.func @distinct(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
// CHECK-LABEL: func.func @array(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
// CHECK-LABEL: func.func @empty_array(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
// CHECK-LABEL: func.func @nested(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
// CHECK-LABEL: func.func @result(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
// CHECK-LABEL: func.func @shared_mutable_elements(
// CHECK: "sec.struct.construct"
// CHECK: "sec.struct.spread_fields"
// CHECK: "sec.struct.extract"
// CHECK: "sec.struct.replace_field"
