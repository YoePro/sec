// RUN: sec-mlir-opt --sec-resolve-scalar-layout %s | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "schema10-array-scalar-layout-64",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @construct(%first: !sec.uint, %second: !sec.uint) -> !sec.array<!sec.uint, "2"> {
    %value = "sec.array.construct"(%first, %second) <{segment_actions = ["construct-direct", "construct-direct"], segment_kinds = ["element", "element"], segment_lengths = ["1", "1"]}> : (!sec.uint, !sec.uint) -> !sec.array<!sec.uint, "2">
    return %value : !sec.array<!sec.uint, "2">
  }

  func.func @nested(%value: !sec.array<!sec.array<!sec.int, "7">, "5">) -> !sec.array<!sec.array<!sec.int, "7">, "5"> {
    return %value : !sec.array<!sec.array<!sec.int, "7">, "5">
  }
}

// CHECK: func.func @construct(%{{.*}}: ui64, %{{.*}}: ui64) -> !sec.array<ui64, "2">
// CHECK: "sec.array.construct"({{.*}}) {{.*}} : (ui64, ui64) -> !sec.array<ui64, "2">
// CHECK: func.func @nested(%{{.*}}: !sec.array<!sec.array<si64, "7">, "5">) -> !sec.array<!sec.array<si64, "7">, "5">
// CHECK-NOT: !sec.int
// CHECK-NOT: !sec.uint
// CHECK-NOT: unrealized_conversion_cast
