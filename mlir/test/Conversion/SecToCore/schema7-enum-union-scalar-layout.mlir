// RUN: sec-mlir-opt --sec-lower-scalar-core %s | FileCheck %s

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>,
  sec.dialect_version = 7 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "enum-union-scalar-layout",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "armv7",
  sec.target_triple = "armv7-unknown-linux-gnueabihf",
  sec.target_abi = "eabihf",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @default_enum(%value: !sec.enum<identity = "main::State", underlying = !sec.int, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "ready", value = "0">]>) -> !sec.enum<identity = "main::State", underlying = !sec.int, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "ready", value = "0">]> {
    return %value : !sec.enum<identity = "main::State", underlying = !sec.int, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "ready", value = "0">]>
  }

  func.func @option(%value: !sec.int) -> !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]> {
    %some = "sec.union.construct"(%value) <{fieldNames = [], payloadActions = ["copy-trivial"], variant = 0 : i32}> : (!sec.int) -> !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
    return %some : !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
  }
}

// CHECK: func.func @default_enum(%arg0: !sec.enum<identity = "main::State", underlying = si32
// CHECK: func.func @option(%arg0: si32) -> !sec.union<identity = "main::Option<int>", typeArguments = [si32]
// CHECK: "sec.union.construct"(%arg0)
// CHECK-SAME: (si32) -> !sec.union<identity = "main::Option<int>", typeArguments = [si32]
// CHECK-NOT: builtin.unrealized_conversion_cast
