// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s

// P17-49: schema 13 lets an ordinary value move into a direct callee, but a
// foreign call never consumes an owned argument (sec_mlir_dialect_v13.md
// section 25; rules/platform/ffi.md sections 35-36).
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "foreign-consumption", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func private @foreign(i32) attributes {sec.extern = true}
  func.func @invalid(%value: i32) {
    "sec.call.foreign"(%value) <{callee = @foreign}> {sec.argument_actions = ["move"]} : (i32) -> ()
    return
  }
}

// CHECK: foreign call cannot consume an owned argument
