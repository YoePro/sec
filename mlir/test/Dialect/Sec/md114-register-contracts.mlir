// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// RUN: sec-mlir-opt %s --sec-lower-scalar-core -o /dev/null
// RUN: sec-mlir-opt %s --sec-lower-integer-core -o /dev/null
// RUN: sed 's/sec.dialect_version = 11/sec.dialect_version = 12/' %s | sec-mlir-opt -o /dev/null
// MD-114: nominal forward references, endian metadata and composed pure paths.
// CHECK: #sec.register_def<identity = "m::Outer"
// CHECK: byte_order = "big-endian"
// CHECK: #sec.register_def<identity = "m::Inner"
// CHECK: byte_order = "little-endian"
// CHECK: sec.field_path = [0 : i32, 1 : i32]
// CHECK: #sec.register_write_proof<basis = "interval", low = "0", high = "7">
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Outer", width = 8, order = "msb-first", byte_order = "big-endian", fields = [#sec.register_field<ordinal = 0, name = "Inner", offset = 4, width = 4, type = !sec.register<"m::Inner">, access = "read-write", shadow = false, reserved = false>, #sec.register_field<ordinal = 1, name = "_", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = true>]>, #sec.register_def<identity = "m::Inner", width = 4, order = "lsb-first", byte_order = "little-endian", fields = [#sec.register_field<ordinal = 0, name = "Flag", offset = 0, width = 1, type = i1, access = "read-write", shadow = false, reserved = false>, #sec.register_field<ordinal = 1, name = "Bits", offset = 1, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @nested(%x: !sec.register<"m::Outer">, %flag: i1) -> !sec.register<"m::Outer"> {
    %bits = "sec.register.extract"(%x) {sec.field_path = [0 : i32, 1 : i32]} : (!sec.register<"m::Outer">) -> ui8
    %updated = "sec.register.update"(%x, %bits) {sec.field_path = [0 : i32, 1 : i32], sec.write_proof = #sec.register_write_proof<basis = "interval", low = "0", high = "7">} : (!sec.register<"m::Outer">, ui8) -> !sec.register<"m::Outer">
    %withFlag = "sec.register.update"(%updated, %flag) {sec.field_path = [0 : i32, 0 : i32], sec.write_proof = #sec.register_write_proof<basis = "typed">} : (!sec.register<"m::Outer">, i1) -> !sec.register<"m::Outer">
    return %withFlag : !sec.register<"m::Outer">
  }

}
