// RUN: not sec-mlir-opt %s -split-input-file --sec-verify-dynamic-arrays -o /dev/null 2>&1 | FileCheck %s

// SEC-MLIR-P18 sections 37, 63-67: the whole-function growth protocol.

// CHECK: must follow the failed state test of the same owner
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @untested() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    cf.br ^grow
  ^grow:
    %f, %r = "sec.dynamic_array.grow_backing"(%p) <{count = "1", transition = "advance-epoch", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> (i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %f, ^done, ^done
  ^done:
    return
  }
}

// -----

// CHECK: is reachable without ensured capacity
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unensured(%x: !sec.int) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    cf.br ^append
  ^append:
    "sec.dynamic_array.append_prepared"(%p, %x) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> ()
    return
  }
}

// -----

// CHECK: candidate is used on its failure edge
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @escape() {
    %a, %failed, %reason = "sec.dynamic_array.allocate"() <{count = "4", transition = "establish-domain", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : () -> (!sec.dynamic_array<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    %b = "sec.own.move"(%a) : (!sec.dynamic_array<!sec.int>) -> !sec.dynamic_array<!sec.int>
    return
  ^success:
    return
  }
}
