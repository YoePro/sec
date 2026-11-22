// RUN: sec-mlir-opt %s --split-input-file --sec-fold-struct-projections | FileCheck %s
// MD-143 § 113B(2): a copy-trivial projection of a constant construction
// (BuildInfo.Current()) folds to the field operand; the dead construction
// tree disappears. Owner-moving constructions are left for the entry gate.

!O = !sec.union<identity = "core::Option<bool>", typeArguments = [i1], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = i1, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
!S = !sec.struct<identity = "core::BuildInfo", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Name", type = !sec.string, tags = []>, #sec.struct_field<ordinal = 1, name = "Build", type = ui64, tags = []>, #sec.struct_field<ordinal = 2, name = "Dirty", type = !O, tags = []>]>

// CHECK-LABEL: func.func @folds
// CHECK-NOT: sec.const.string
// CHECK-NOT: sec.union.construct
// CHECK-NOT: sec.struct
// CHECK: %[[BUILD:.*]] = "sec.const.int"() <{value = "10"}> : () -> ui64
// CHECK: return %[[BUILD]] : ui64
func.func @folds() -> ui64 {
  %name = "sec.const.string"() <{value = "demo"}> : () -> !sec.string
  %build = "sec.const.int"() <{value = "10"}> : () -> ui64
  %dirty = "sec.union.construct"() <{fieldNames = [], payloadActions = [], variant = 1 : i32}> : () -> !O
  %info = "sec.struct.construct"(%name, %build, %dirty) <{field_actions = ["copy-trivial", "copy-trivial", "copy-trivial"], field_origins = ["explicit", "explicit", "explicit"]}> : (!sec.string, ui64, !O) -> !S
  %field = "sec.struct.extract"(%info) <{action = "copy-trivial", field = 1 : i32}> : (!S) -> ui64
  return %field : ui64
}

// -----

!P = !sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "name", type = !sec.string, tags = []>, #sec.struct_field<ordinal = 1, name = "count", type = ui64, tags = []>]>

// CHECK-LABEL: func.func @keeps_owner_moves
// CHECK: sec.struct.construct
// CHECK: sec.struct.extract
func.func @keeps_owner_moves() -> ui64 {
  %name = "sec.const.string"() <{value = "a"}> : () -> !sec.string
  %count = "sec.const.int"() <{value = "7"}> : () -> ui64
  %pair = "sec.struct.construct"(%name, %count) <{field_actions = ["construct-direct", "construct-direct"], field_origins = ["explicit", "explicit"]}> : (!sec.string, ui64) -> !P
  %field = "sec.struct.extract"(%pair) <{action = "copy-trivial", field = 1 : i32}> : (!P) -> ui64
  return %field : ui64
}
