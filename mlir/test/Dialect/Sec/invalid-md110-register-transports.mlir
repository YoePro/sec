// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-110 v11.41.2 amendment: forged guarded and bit-extraction proofs and
// malformed fits_bits / int.bit_extract fail closed.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @unguarded(%x: si32) -> !sec.register<"m::Nib"> {
    // expected-error@+1 {{register guarded bit-fit proof requires a dominating successful register.fits_bits}}
    %r = "sec.register.from_bits"(%x) {sec.register_fit_proof = {basis = "guarded"}} : (si32) -> !sec.register<"m::Nib">
    return %r : !sec.register<"m::Nib">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @false_edge(%x: si32) -> !sec.register<"m::Nib"> {
    %ok = "sec.register.fits_bits"(%x) <{register_type = !sec.register<"m::Nib">}> : (si32) -> i1
    cf.cond_br %ok, ^fits, ^fail
  ^fits:
    %low = "sec.int.bit_extract"(%x) <{width = 4 : i32}> : (si32) -> ui8
    %wrapped = "sec.register.from_bits"(%low) {sec.register_fit_proof = {basis = "bit-extraction"}} : (ui8) -> !sec.register<"m::Nib">
    return %wrapped : !sec.register<"m::Nib">
  ^fail:
    // expected-error@+1 {{register guarded bit-fit proof requires a dominating successful register.fits_bits}}
    %r = "sec.register.from_bits"(%x) {sec.register_fit_proof = {basis = "guarded"}} : (si32) -> !sec.register<"m::Nib">
    return %r : !sec.register<"m::Nib">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @not_register(%x: si32) -> i1 {
    // expected-error@+1 {{register.fits_bits requires register_type naming a defined register}}
    %ok = "sec.register.fits_bits"(%x) <{register_type = ui8}> : (si32) -> i1
    return %ok : i1
  }
}

// -----

// expected-error@+1 {{register type has no definition}}
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @undefined_register(%x: si32) -> i1 {
    %ok = "sec.register.fits_bits"(%x) <{register_type = !sec.register<"m::Missing">}> : (si32) -> i1
    return %ok : i1
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @zero_width(%x: si32) -> ui8 {
    // expected-error@+1 {{int.bit_extract width must be between 1 and the operand width}}
    %b = "sec.int.bit_extract"(%x) <{width = 0 : i32}> : (si32) -> ui8
    return %b : ui8
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @too_wide(%x: si8) -> ui16 {
    // expected-error@+1 {{int.bit_extract width must be between 1 and the operand width}}
    %b = "sec.int.bit_extract"(%x) <{width = 9 : i32}> : (si8) -> ui16
    return %b : ui16
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @wrong_carrier(%x: si32) -> ui16 {
    // expected-error@+1 {{int.bit_extract result must be the smallest standard unsigned integer holding width bits}}
    %b = "sec.int.bit_extract"(%x) <{width = 4 : i32}> : (si32) -> ui16
    return %b : ui16
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @not_extracted(%x: ui8) -> !sec.register<"m::Nib"> {
    // expected-error@+1 {{register bit-extraction proof requires the direct result of an int.bit_extract no wider than the register}}
    %r = "sec.register.from_bits"(%x) {sec.register_fit_proof = {basis = "bit-extraction"}} : (ui8) -> !sec.register<"m::Nib">
    return %r : !sec.register<"m::Nib">
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @wider_than_register(%x: si32) -> !sec.register<"m::Nib"> {
    %b = "sec.int.bit_extract"(%x) <{width = 5 : i32}> : (si32) -> ui8
    // expected-error@+1 {{register bit-extraction proof requires the direct result of an int.bit_extract no wider than the register}}
    %r = "sec.register.from_bits"(%b) {sec.register_fit_proof = {basis = "bit-extraction"}} : (ui8) -> !sec.register<"m::Nib">
    return %r : !sec.register<"m::Nib">
  }
}

// -----

// expected-error@+1 {{int.bit_extract requires schema 11, module declares schema 10}}
module attributes {sec.dialect_version = 10 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @old(%x: si32) -> ui8 {
    %b = "sec.int.bit_extract"(%x) <{width = 4 : i32}> : (si32) -> ui8
    return %b : ui8
  }
}
