// RUN: not sec-mlir-opt %s 2>&1 | FileCheck %s
// A schema-12 slice is a bounded reference: it is passed with the
// shared-reference / mutable-reference action of its authority, never copied
// as an ordinary value (sec_mlir_dialect.md v12.5; P16 sections 64-65).
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "slice-call-action",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func private @target(!sec.slice_mut<!sec.int>) attributes {sec.extern = false}
  func.func @invalid(%values: !sec.slice_mut<!sec.int>) {
    "sec.call.direct"(%values) <{callee = @target}> {sec.argument_actions = ["copy-trivial"]} : (!sec.slice_mut<!sec.int>) -> ()
    return
  }
}

// CHECK: argument action must be mutable-reference
