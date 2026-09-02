// RUN: sec-mlir-opt --sec-resolve-scalar-layout %s | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema10-array-scalar-layout-32",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "armv7",
  sec.target_triple = "armv7-unknown-linux-gnueabihf",
  sec.target_abi = "eabihf",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @construct(%first: !sec.int, %second: !sec.int) -> !sec.array<!sec.int, "2"> {
    %value = "sec.array.construct"(%first, %second) <{segment_actions = ["construct-direct", "construct-direct"], segment_kinds = ["element", "element"], segment_lengths = ["1", "1"]}> : (!sec.int, !sec.int) -> !sec.array<!sec.int, "2">
    return %value : !sec.array<!sec.int, "2">
  }

  func.func @wrapped(
      %value: !sec.struct<identity = "main::Packet", typeArguments = [!sec.array<!sec.uint, "4">], fields = [#sec.struct_field<ordinal = 0, name = "lanes", type = !sec.array<!sec.array<!sec.uint, "4">, "3">, tags = [#sec.struct_tag<key = "unit", value = "words">]>]>)
      -> !sec.struct<identity = "main::Packet", typeArguments = [!sec.array<!sec.uint, "4">], fields = [#sec.struct_field<ordinal = 0, name = "lanes", type = !sec.array<!sec.array<!sec.uint, "4">, "3">, tags = [#sec.struct_tag<key = "unit", value = "words">]>]> {
    return %value : !sec.struct<identity = "main::Packet", typeArguments = [!sec.array<!sec.uint, "4">], fields = [#sec.struct_field<ordinal = 0, name = "lanes", type = !sec.array<!sec.array<!sec.uint, "4">, "3">, tags = [#sec.struct_tag<key = "unit", value = "words">]>]>
  }
}

// CHECK: func.func @construct(%{{.*}}: si32, %{{.*}}: si32) -> !sec.array<si32, "2">
// CHECK: "sec.array.construct"({{.*}}) {{.*}} : (si32, si32) -> !sec.array<si32, "2">
// CHECK: !sec.struct<identity = "main::Packet", typeArguments = [!sec.array<ui32, "4">]
// CHECK-SAME: type = !sec.array<!sec.array<ui32, "4">, "3">
// CHECK-SAME: #sec.struct_tag<key = "unit", value = "words">
// CHECK-NOT: !sec.int
// CHECK-NOT: !sec.uint
// CHECK-NOT: unrealized_conversion_cast
