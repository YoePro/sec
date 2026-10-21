// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// P16 sections 88-90: identity is element type and authority alone.
// Wide, nominal, nested and reference elements retain their semantic identity;
// no physical descriptor, length, origin or epoch is a type argument.
// CHECK: sec.dialect_version = 12 : i32
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK-LABEL: func.func private @shared(!sec.slice<!sec.int>) -> !sec.slice<!sec.int>
  func.func private @shared(!sec.slice<!sec.int>) -> !sec.slice<!sec.int>
  // CHECK-LABEL: func.func private @mutable(!sec.slice_mut<!sec.int>) -> !sec.slice_mut<!sec.int>
  func.func private @mutable(!sec.slice_mut<!sec.int>) -> !sec.slice_mut<!sec.int>
  // CHECK-LABEL: func.func private @wide(!sec.slice<si256>) -> !sec.slice<si256>
  func.func private @wide(!sec.slice<si256>) -> !sec.slice<si256>
  // CHECK-LABEL: func.func private @decimal(!sec.slice_mut<!sec.decimal128>) -> !sec.slice_mut<!sec.decimal128>
  func.func private @decimal(!sec.slice_mut<!sec.decimal128>) -> !sec.slice_mut<!sec.decimal128>
  // CHECK-LABEL: func.func private @named(!sec.slice<!sec.named<"main::Score", si128>>) -> !sec.slice<!sec.named<"main::Score", si128>>
  func.func private @named(!sec.slice<!sec.named<"main::Score", si128>>) -> !sec.slice<!sec.named<"main::Score", si128>>
  // CHECK-LABEL: func.func private @distinct(!sec.slice_mut<!sec.distinct<"main::Score", si128>>) -> !sec.slice_mut<!sec.distinct<"main::Score", si128>>
  func.func private @distinct(!sec.slice_mut<!sec.distinct<"main::Score", si128>>) -> !sec.slice_mut<!sec.distinct<"main::Score", si128>>
  // CHECK-LABEL: func.func private @array(!sec.slice<!sec.array<si128, "4">>) -> !sec.slice<!sec.array<si128, "4">>
  func.func private @array(!sec.slice<!sec.array<si128, "4">>) -> !sec.slice<!sec.array<si128, "4">>
  // CHECK-LABEL: func.func private @nested(!sec.slice<!sec.slice<!sec.int>>) -> !sec.slice<!sec.slice<!sec.int>>
  func.func private @nested(!sec.slice<!sec.slice<!sec.int>>) -> !sec.slice<!sec.slice<!sec.int>>
  // CHECK-LABEL: func.func private @references(!sec.slice<!sec.ref<si128>>) -> !sec.slice<!sec.ref<si128>>
  func.func private @references(!sec.slice<!sec.ref<si128>>) -> !sec.slice<!sec.ref<si128>>
  // CHECK-LABEL: func.func private @structure(
  // CHECK-SAME: !sec.slice<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Value", type = si128, tags = []>]>>
  func.func private @structure(!sec.slice<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Value", type = si128, tags = []>]>>) -> !sec.slice<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Value", type = si128, tags = []>]>>

}
