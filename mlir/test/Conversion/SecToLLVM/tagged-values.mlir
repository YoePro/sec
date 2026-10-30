// RUN: sec-mlir-opt %s --split-input-file --sec-lower-to-llvm=entry= | FileCheck %s
// Rules: rules/mlir/sec_mlir_lowering.md — v11.33; rules/memory/layout.md §§ 15-16.

!U = !sec.union<identity = "main::Figure", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Box", kind = "fields", fields = [#sec.union_field<name = "Width", type = !sec.int>, #sec.union_field<name = "Height", type = !sec.int>]>, #sec.union_variant<index = 1, name = "Dot", kind = "empty", fields = []>, #sec.union_variant<index = 2, name = "Count", kind = "single", payload = i8, fields = []>]>

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
// The discriminator is the declaration-order index, followed by one slot per
// payload position in declaration order. Only the active slots are written.
// CHECK-LABEL: llvm.func @make_count
// CHECK-DAG: %[[TAG:.*]] = llvm.mlir.constant(2 : i32) : i32
// CHECK-DAG: %[[UNDEF:.*]] = llvm.mlir.undef : !llvm.struct<(i32, i64, i64, i8)>
// CHECK: %[[TAGGED:.*]] = llvm.insertvalue %[[TAG]], %[[UNDEF]][0]
// CHECK: llvm.insertvalue %{{.*}}, %[[TAGGED]][3]
// CHECK-NOT: llvm.insertvalue
// CHECK: llvm.return
func.func @make_count(%value: i8) -> !U {
  %made = "sec.union.construct"(%value) <{fieldNames = [], payloadActions = ["copy-trivial"], variant = 2 : i32}> : (i8) -> !U
  return %made : !U
}

// A variant test compares the discriminator, and a field projection reads its
// own slot on the guarded edge.
// CHECK-LABEL: llvm.func @box_height
// CHECK: %[[KIND:.*]] = llvm.extractvalue %{{.*}}[0] : !llvm.struct<(i32, i64, i64, i8)>
// CHECK: %[[WANT:.*]] = llvm.mlir.constant(0 : i32) : i32
// CHECK: llvm.icmp "eq" %[[KIND]], %[[WANT]] : i32
// CHECK: llvm.extractvalue %{{.*}}[2] : !llvm.struct<(i32, i64, i64, i8)>
func.func @box_height(%shape: !U, %other: !sec.int) -> !sec.int {
  %is = "sec.union.is_variant"(%shape) <{variant = 0 : i32}> : (!U) -> i1
  cf.cond_br %is, ^box, ^rest
^box:
  %height = "sec.union.unwrap_field"(%shape) <{field = "Height", payloadAction = "copy-trivial", variant = 0 : i32}> : (!U) -> !sec.int
  return %height : !sec.int
^rest:
  return %other : !sec.int
}
}

// -----

!R = !sec.result<!sec.int, i8>

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
// Err is the set i1 discriminator; Ok and Err each own a slot.
// CHECK-LABEL: llvm.func @fail
// CHECK: llvm.mlir.constant(true) : i1
// CHECK: llvm.insertvalue %{{.*}}[2] : !llvm.struct<(i1, i64, i8)>
func.func @fail(%code: i8) -> !R {
  %failed = "sec.result.err"(%code) : (i8) -> !R
  return %failed : !R
}

// CHECK-LABEL: llvm.func @either
// CHECK: llvm.extractvalue %{{.*}}[0] : !llvm.struct<(i1, i64, i8)>
// CHECK: llvm.extractvalue %{{.*}}[1] : !llvm.struct<(i1, i64, i8)>
func.func @either(%value: !R, %fallback: !sec.int) -> !sec.int {
  %is_err = "sec.result.is_err"(%value) : (!R) -> i1
  cf.cond_br %is_err, ^err, ^ok
^err:
  %code = "sec.result.unwrap_err"(%value) : (!R) -> i8
  return %fallback : !sec.int
^ok:
  %found = "sec.result.unwrap_ok"(%value) : (!R) -> !sec.int
  return %found : !sec.int
}
}

// -----

!U = !sec.union<identity = "main::Flag", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "On", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Off", kind = "empty", fields = []>]>

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
// Mutable storage is one stack slot, allocated in the entry block.
// CHECK-LABEL: llvm.func @keep
// CHECK-NEXT: llvm.mlir.constant(1 : i64)
// CHECK-NEXT: llvm.alloca %{{.*}} x !llvm.struct<(i32)>
// CHECK: llvm.store %{{.*}}, %{{.*}} : !llvm.struct<(i32)>, !llvm.ptr
// CHECK: llvm.load %{{.*}} : !llvm.ptr -> !llvm.struct<(i32)>
func.func @keep(%flag: !U, %go: i1) -> !U {
  cf.cond_br %go, ^store, ^done(%flag : !U)
^store:
  %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "slot", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!U>
  "sec.storage.init"(%slot, %flag) : (!sec.storage<!U>, !U) -> ()
  %loaded = "sec.storage.load"(%slot) : (!sec.storage<!U>) -> !U
  cf.br ^done(%loaded : !U)
^done(%result: !U):
  return %result : !U
}
}
