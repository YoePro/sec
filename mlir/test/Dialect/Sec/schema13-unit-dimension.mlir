// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

// MD-062 §§ 7(12), 12(8): UnitDimension values. A constant is the canonical
// sorted axis mapping with non-zero exponents; comparison is exact equality
// or inequality of two dimensions.

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "dims", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @use(%d: !sec.unit_dimension) -> i1 {
    %c = "sec.unit_dimension.constant"() <{axes = [{axis = "length", exponent = 1 : i64}, {axis = "time", exponent = -1 : i64}]}> : () -> !sec.unit_dimension
    %e = "sec.unit_dimension.constant"() <{axes = []}> : () -> !sec.unit_dimension
    %n = "sec.unit_dimension.axis_count"(%c) : (!sec.unit_dimension) -> !sec.uint
    %z = "sec.unit_dimension.is_dimensionless"(%e) : (!sec.unit_dimension) -> i1
    %r = "sec.unit_dimension.compare"(%d, %c) <{predicate = "ne"}> : (!sec.unit_dimension, !sec.unit_dimension) -> i1
    return %r : i1
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "dims", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad() {
    // expected-error@+1 {{axes must be sorted and unique by axis identity}}
    %c = "sec.unit_dimension.constant"() <{axes = [{axis = "time", exponent = 1 : i64}, {axis = "length", exponent = 1 : i64}]}> : () -> !sec.unit_dimension
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "dims", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad() {
    // expected-error@+1 {{axis length has a zero exponent}}
    %c = "sec.unit_dimension.constant"() <{axes = [{axis = "length", exponent = 0 : i64}]}> : () -> !sec.unit_dimension
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "dims", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad() {
    // expected-error@+1 {{each axis is exactly {axis, exponent}}}
    %c = "sec.unit_dimension.constant"() <{axes = [{axis = "length"}]}> : () -> !sec.unit_dimension
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "dims", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%d: !sec.unit_dimension) {
    // expected-error@+1 {{predicate must be eq or ne}}
    %r = "sec.unit_dimension.compare"(%d, %d) <{predicate = "lt"}> : (!sec.unit_dimension, !sec.unit_dimension) -> i1
    return
  }
}

// -----

module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "dims", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bad(%d: !sec.int) {
    // expected-error@+1 {{operand must be !sec.unit_dimension}}
    %n = "sec.unit_dimension.axis_count"(%d) : (!sec.int) -> !sec.uint
    return
  }
}
