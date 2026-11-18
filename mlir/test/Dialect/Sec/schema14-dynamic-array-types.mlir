// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

// SEC-MLIR-P18 section 127: !sec.dynamic_array<T> round-trips with a wide
// element, a nested struct element and a nested dynamic-array element.

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "types", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  // CHECK-LABEL: func.func @types
  // CHECK-SAME: !sec.dynamic_array<si128>
  // CHECK-SAME: !sec.dynamic_array<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = ui8, tags = []>]>>
  // CHECK-SAME: !sec.dynamic_array<!sec.dynamic_array<ui8>>
  func.func @types(%wide: !sec.dynamic_array<si128>, %pairs: !sec.dynamic_array<!sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = ui8, tags = []>]>>, %nested: !sec.dynamic_array<!sec.dynamic_array<ui8>>) {
    return
  }
}
