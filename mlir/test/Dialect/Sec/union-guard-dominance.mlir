// RUN: sec-mlir-opt --sec-verify-union-guards %s | FileCheck %s
// A projection dominated by the true edge of its variant test through
// single-predecessor blocks, as after a match guard, is guarded.

!o = !sec.union<identity = "main::Option<int>", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.dialect_version = 7 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "union-guard-dominance", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little"} {
  // CHECK-LABEL: func.func @through_guard
  func.func @through_guard(%value: !o, %guard: i1) -> !sec.int {
    %some = "sec.union.is_variant"(%value) <{variant = 0 : i32}> : (!o) -> i1
    cf.cond_br %some, ^tested, ^other
  ^tested:
    cf.cond_br %guard, ^commit, ^other
  ^commit:
    %payload = "sec.union.unwrap_payload"(%value) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!o) -> !sec.int
    return %payload : !sec.int
  ^other:
    %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    return %zero : !sec.int
  }
}
