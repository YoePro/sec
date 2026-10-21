// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md §§39,114,140.
// Forged copy actions cannot hide exclusive holders inside owned storage.
!field = !sec.slice_mut<si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.slice_mut<si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @direct_direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.slice_mut<si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @direct_spread(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %new = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.slice_mut<si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @direct_extract(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %new = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.slice_mut<si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @direct_replace(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %new = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!holder, !field) -> !holder
    return
  }
}
// -----
!field = !sec.named<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @named_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.named<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @named_direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.named<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @named_spread(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %new = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.named<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @named_extract(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %new = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.named<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @named_replace(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %new = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!holder, !field) -> !holder
    return
  }
}
// -----
!field = !sec.distinct<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @distinct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.distinct<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @distinct_direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.distinct<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @distinct_spread(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %new = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.distinct<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @distinct_extract(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %new = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.distinct<"main::View", !sec.slice_mut<si32>>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @distinct_replace(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %new = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!holder, !field) -> !holder
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "2">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @array_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "2">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @array_direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "2">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @array_spread(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %new = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "2">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @array_extract(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %new = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "2">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @array_replace(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %new = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!holder, !field) -> !holder
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "0">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @empty_array_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "0">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @empty_array_direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "0">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @empty_array_spread(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %new = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "0">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @empty_array_extract(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %new = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.array<!sec.slice_mut<si32>, "0">
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @empty_array_replace(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %new = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!holder, !field) -> !holder
    return
  }
}
// -----
!field = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !sec.slice_mut<si32>, tags = []>]>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @nested_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !sec.slice_mut<si32>, tags = []>]>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @nested_direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !sec.slice_mut<si32>, tags = []>]>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @nested_spread(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %new = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !sec.slice_mut<si32>, tags = []>]>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @nested_extract(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %new = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.struct<identity = "main::Inner", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !sec.slice_mut<si32>, tags = []>]>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @nested_replace(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %new = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!holder, !field) -> !holder
    return
  }
}
// -----
!field = !sec.result<!sec.slice_mut<si32>, si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @result_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.result<!sec.slice_mut<si32>, si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @result_direct_construct(%holder: !holder, %field: !field) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %new = "sec.struct.construct"(%field) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (!field) -> !holder
    return
  }
}
// -----
!field = !sec.result<!sec.slice_mut<si32>, si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @result_spread(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %new = "sec.struct.spread_fields"(%holder) <{actions = ["copy-trivial"]}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.result<!sec.slice_mut<si32>, si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @result_extract(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %new = "sec.struct.extract"(%holder) <{action = "copy-trivial", field = 0 : i32}> : (!holder) -> !field
    return
  }
}
// -----
!field = !sec.result<!sec.slice_mut<si32>, si32>
!holder = !sec.struct<identity = "main::Views", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Data", type = !field, tags = []>]>
module attributes {sec.dialect_version = 12 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "slice-aggregate", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @result_replace(%holder: !holder, %field: !field) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %new = "sec.struct.replace_field"(%holder, %field) <{field = 0 : i32}> : (!holder, !field) -> !holder
    return
  }
}
