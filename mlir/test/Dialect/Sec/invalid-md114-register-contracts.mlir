// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-114 §§2–7,11: table/path/type/proof forgeries fail closed.
// expected-error@+1 {{register type has no definition}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func private @missing(!sec.register<"m::R">)
}

// -----

// expected-error@+1 {{unique typed register definitions}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>, #sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{ordinals/layout must be contiguous}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 1, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{cover the complete width}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{offset disagrees}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 1, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+2 {{reserved flag must agree}}
// expected-error@+1 {{failed to parse Sec_RegisterDefAttr parameter}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "_", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{smallest unsigned carrier}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui16, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+2 {{invalid register field}}
// expected-error@+1 {{failed to parse Sec_RegisterDefAttr parameter}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "unknown", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{matching definition and width}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = !sec.register<"m::Missing">, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{recursive register layout}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = !sec.register<"m::R">, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{requires schema 11}}
module attributes {sec.dialect_version = 10 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

// expected-error@+1 {{duplicate or unknown struct parameter name: surprise}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [], surprise = true>]} {

}

// -----

// expected-error@+1 {{invalid register byte_order}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", byte_order = "native", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {

}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @empty(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{nonempty sec.field_path}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [], sec.write_proof = #sec.register_write_proof<basis = "constant", low = "0", high = "0">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @unknown(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{invalid register field ordinal}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [1 : i32], sec.write_proof = #sec.register_write_proof<basis = "constant", low = "0", high = "0">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @scalar(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{descends into a scalar}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32, 0 : i32], sec.write_proof = #sec.register_write_proof<basis = "constant", low = "0", high = "0">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @typed(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{typed proof cannot justify}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "typed">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @forged(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{not established by replacement SSA}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "constant", low = "1", high = "1">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @range(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{not established by replacement SSA}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "interval", low = "0", high = "7">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @overflow(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{exceeds selected field width}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "constant", low = "8", high = "8">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @carrier(%x: !sec.register<"m::R">, %v: i1) -> !sec.register<"m::R"> {
    // expected-error@+1 {{exact field carrier}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "typed">} : (!sec.register<"m::R">, i1) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @proof(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{typed register write proof cannot carry bounds}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "typed", low = "0", high = "0">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @proof(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{canonical ordered nonnegative bounds}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "constant", low = "01", high = "01">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @proof(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{bounds must be equal}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "constant", low = "1", high = "2">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @proof(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{canonical ordered nonnegative bounds}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "interval", low = "2", high = "1">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @proof(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{canonical ordered nonnegative bounds}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "interval", low = "-1", high = "1">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @proof(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{canonical ordered nonnegative bounds}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "interval", low = "0">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::R", width = 3, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 3, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @proof(%x: !sec.register<"m::R">, %v: ui8) -> !sec.register<"m::R"> {
    // expected-error@+1 {{unknown register write proof basis}}
    %r = "sec.register.update"(%x, %v) {sec.field_path = [0 : i32], sec.write_proof = #sec.register_write_proof<basis = "source-domain">} : (!sec.register<"m::R">, ui8) -> !sec.register<"m::R">
    return %r : !sec.register<"m::R">
  }
}
