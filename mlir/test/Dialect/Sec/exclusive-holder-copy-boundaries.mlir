// RUN: sec-mlir-opt %s | sec-mlir-opt -o /dev/null
// Shared holders and explicit mutable-reference argument actions remain valid.
module attributes {
  sec.dialect_version = 12 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "exclusive-copy-boundaries",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  func.func @shared_0(%p: !sec.storage<!sec.ref<si32>>) -> !sec.ref<si32> {
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.ref<si32>>) -> !sec.ref<si32>
    return %copy : !sec.ref<si32>
  }
  func.func @shared_1(%p: !sec.storage<!sec.ref<!sec.slice_mut<si32>>>) -> !sec.ref<!sec.slice_mut<si32>> {
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.ref<!sec.slice_mut<si32>>>) -> !sec.ref<!sec.slice_mut<si32>>
    return %copy : !sec.ref<!sec.slice_mut<si32>>
  }
  func.func @shared_2(%p: !sec.storage<!sec.slice<!sec.ref_mut<si32>>>) -> !sec.slice<!sec.ref_mut<si32>> {
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.slice<!sec.ref_mut<si32>>>) -> !sec.slice<!sec.ref_mut<si32>>
    return %copy : !sec.slice<!sec.ref_mut<si32>>
  }
  func.func @shared_3(%p: !sec.storage<!sec.slice<!sec.slice_mut<si32>>>) -> !sec.slice<!sec.slice_mut<si32>> {
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.slice<!sec.slice_mut<si32>>>) -> !sec.slice<!sec.slice_mut<si32>>
    return %copy : !sec.slice<!sec.slice_mut<si32>>
  }
  func.func @shared_4(%p: !sec.storage<!sec.array<!sec.ref<si32>, "0">>) -> !sec.array<!sec.ref<si32>, "0"> {
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.array<!sec.ref<si32>, "0">>) -> !sec.array<!sec.ref<si32>, "0">
    return %copy : !sec.array<!sec.ref<si32>, "0">
  }
  func.func private @target(!sec.ref_mut<si32>, !sec.slice_mut<si32>) attributes {sec.extern = false}
  func.func @exclusive_arguments(%r: !sec.ref_mut<si32>, %s: !sec.slice_mut<si32>) {
    "sec.call.direct"(%r, %s) <{callee = @target}> {sec.argument_actions = ["mutable-reference", "mutable-reference"]} : (!sec.ref_mut<si32>, !sec.slice_mut<si32>) -> ()
    return
  }
}
