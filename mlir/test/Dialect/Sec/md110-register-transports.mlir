// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// RUN: sed 's/sec.dialect_version = 11/sec.dialect_version = 12/' %s | sec-mlir-opt -o /dev/null
// RUN: sec-mlir-opt %s --sec-lower-integer-core | FileCheck %s --check-prefix=LOWER
// MD-110 v11.41.2 amendment: fits_bits destination TypeAttr, the guarded basis
// and the bit-extraction transport through sec.int.bit_extract.
// CHECK-LABEL: func.func @guarded
// CHECK: "sec.register.fits_bits"(%{{.*}}) <{register_type = !sec.register<"m::Nib">}> : (si32) -> i1
// CHECK: basis = "guarded"
// CHECK-LABEL: func.func @extracted
// CHECK: "sec.int.bit_extract"(%{{.*}}) <{width = 4 : i32}> : (si32) -> ui8
// CHECK: basis = "bit-extraction"
// LOWER-LABEL: func.func @low_bits
// LOWER: arith.trunci %{{.*}} : i32 to i8
// LOWER: arith.andi
// LOWER-NOT: sec.int.bit_extract
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "registers", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.registers = [#sec.register_def<identity = "m::Nib", width = 4, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Bits", offset = 0, width = 4, type = ui8, access = "read-write", shadow = false, reserved = false>]>]} {
  func.func @guarded(%x: si32) -> !sec.register<"m::Nib"> {
    %ok = "sec.register.fits_bits"(%x) <{register_type = !sec.register<"m::Nib">}> : (si32) -> i1
    cf.cond_br %ok, ^fits, ^fail
  ^fits:
    %r = "sec.register.from_bits"(%x) {sec.register_fit_proof = {basis = "guarded"}} : (si32) -> !sec.register<"m::Nib">
    return %r : !sec.register<"m::Nib">
  ^fail:
    %low = "sec.int.bit_extract"(%x) <{width = 4 : i32}> : (si32) -> ui8
    %wrapped = "sec.register.from_bits"(%low) {sec.register_fit_proof = {basis = "bit-extraction"}} : (ui8) -> !sec.register<"m::Nib">
    return %wrapped : !sec.register<"m::Nib">
  }
  func.func @extracted(%x: si32) -> !sec.register<"m::Nib"> {
    %b = "sec.int.bit_extract"(%x) <{width = 4 : i32}> : (si32) -> ui8
    %r = "sec.register.from_bits"(%b) {sec.register_fit_proof = {basis = "bit-extraction"}} : (ui8) -> !sec.register<"m::Nib">
    return %r : !sec.register<"m::Nib">
  }
  func.func @low_bits(%x: si32) -> ui8 {
    %b = "sec.int.bit_extract"(%x) <{width = 4 : i32}> : (si32) -> ui8
    return %b : ui8
  }
}
