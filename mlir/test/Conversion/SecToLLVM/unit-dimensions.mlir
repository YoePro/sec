// RUN: sec-mlir-opt %s --split-input-file --sec-lower-to-llvm=entry= | FileCheck %s
// RUN: sec-mlir-opt %s --split-input-file --sec-lower-to-llvm=entry= | FileCheck %s --check-prefix=EQ
// Rules: rules/corrections/applied/md062-unit-metadata-value-types-correction-20261027-20261121.md — §§ 7(5)-(7), 7(12)-(13).

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
// A constant addresses one internal descriptor per canonical mapping: the
// collision-free length-prefixed axis encoding, shared by equal constants.
// CHECK-DAG: llvm.mlir.global internal constant @__sec_unit_dimension.{{[0-9]+}}("6:length1;4:time-1;")
// CHECK-DAG: llvm.mlir.global internal constant @__sec_unit_dimension.{{[0-9]+}}("6:length1;")
// CHECK-LABEL: llvm.func @speed
// CHECK: llvm.mlir.addressof @__sec_unit_dimension
// CHECK: llvm.mlir.undef : !llvm.struct<(ptr, i64, i64)>
// CHECK: llvm.mlir.constant(19 : i64)
// CHECK: llvm.mlir.constant(2 : i64)
func.func @speed() -> !sec.unit_dimension {
  %c = "sec.unit_dimension.constant"() <{axes = [{axis = "length", exponent = 1 : i64}, {axis = "time", exponent = -1 : i64}]}> : () -> !sec.unit_dimension
  return %c : !sec.unit_dimension
}

func.func @length() -> !sec.unit_dimension {
  %c = "sec.unit_dimension.constant"() <{axes = [{axis = "length", exponent = 1 : i64}]}> : () -> !sec.unit_dimension
  %d = "sec.unit_dimension.constant"() <{axes = [{axis = "length", exponent = 1 : i64}]}> : () -> !sec.unit_dimension
  return %d : !sec.unit_dimension
}

// The dimensionless mapping has no descriptor.
// CHECK-LABEL: llvm.func @plain
// CHECK: llvm.mlir.zero : !llvm.ptr
// CHECK-NOT: llvm.mlir.addressof
// CHECK: llvm.return
func.func @plain() -> !sec.unit_dimension {
  %c = "sec.unit_dimension.constant"() <{axes = []}> : () -> !sec.unit_dimension
  return %c : !sec.unit_dimension
}

// Equality of parameters is a byte comparison of the descriptors through the
// internal helper; inequality is its negation. No pointer comparison.
// EQ-LABEL: llvm.func @different
// EQ: llvm.call @__sec_unit_dimension_equal
// EQ: llvm.xor
// EQ-LABEL: llvm.func internal @__sec_unit_dimension_equal(%{{.*}}: !llvm.ptr, %{{.*}}: i64, %{{.*}}: !llvm.ptr, %{{.*}}: i64) -> i1
// EQ: llvm.icmp "eq" %{{.*}}, %{{.*}} : i64
// EQ: llvm.load %{{.*}} : !llvm.ptr -> i8
// EQ: llvm.load %{{.*}} : !llvm.ptr -> i8
// EQ: llvm.icmp "eq" %{{.*}}, %{{.*}} : i8
func.func @different(%a: !sec.unit_dimension, %b: !sec.unit_dimension) -> i1 {
  %r = "sec.unit_dimension.compare"(%a, %b) <{predicate = "ne"}> : (!sec.unit_dimension, !sec.unit_dimension) -> i1
  return %r : i1
}

// AxisCount reads the count slot at the plan's uint width; IsDimensionless
// tests it for zero.
// CHECK-LABEL: llvm.func @count
// CHECK: llvm.extractvalue %{{.*}}[2] : !llvm.struct<(ptr, i64, i64)>
// CHECK: llvm.return %{{.*}} : i64
func.func @count(%a: !sec.unit_dimension) -> !sec.uint {
  %n = "sec.unit_dimension.axis_count"(%a) : (!sec.unit_dimension) -> !sec.uint
  return %n : !sec.uint
}

// CHECK-LABEL: llvm.func @dimensionless
// CHECK: llvm.extractvalue %{{.*}}[2] : !llvm.struct<(ptr, i64, i64)>
// CHECK: llvm.icmp "eq"
func.func @dimensionless(%a: !sec.unit_dimension) -> i1 {
  %z = "sec.unit_dimension.is_dimensionless"(%a) : (!sec.unit_dimension) -> i1
  return %z : i1
}
}

// -----

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
// On a 32-bit plan AxisCount truncates the bounded count to the plan's uint.
// CHECK-LABEL: llvm.func @count32
// CHECK: llvm.extractvalue %{{.*}}[2] : !llvm.struct<(ptr, i64, i64)>
// CHECK: llvm.trunc %{{.*}} : i64 to i32
func.func @count32(%a: !sec.unit_dimension) -> !sec.uint {
  %n = "sec.unit_dimension.axis_count"(%a) : (!sec.unit_dimension) -> !sec.uint
  return %n : !sec.uint
}
}
