// RUN: not sec-mlir-opt %s -split-input-file -o /dev/null 2>&1 | FileCheck %s

// CHECK: sec.destruction_plans requires a schema-13 module
module attributes {sec.destruction_plans = []} {
}

// -----

// CHECK: sec.destruction_plans must be an array of dictionaries
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "plans", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>, sec.destruction_plans = ["x"]} {
}
