// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// MD-077 sections 3(5), 19(10): software Places may be borrowed normally.
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "md077-software-references",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK-LABEL: func.func @shared
  func.func @shared(%p: !sec.place<!sec.hw_instance<"device::Control">, "ro">) {
    // CHECK: sec.ref.borrow_shared
    %ref = "sec.ref.borrow_shared"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.hw_instance<"device::Control">, "ro">) -> !sec.ref<!sec.hw_instance<"device::Control">>
    // CHECK: sec.ref.copy_shared
    %next = "sec.ref.copy_shared"(%ref) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref<!sec.hw_instance<"device::Control">>) -> !sec.ref<!sec.hw_instance<"device::Control">>
    // CHECK: sec.place.deref
    %back = "sec.place.deref"(%next) : (!sec.ref<!sec.hw_instance<"device::Control">>) -> !sec.place<!sec.hw_instance<"device::Control">, "ro">
    return
  }
  // CHECK-LABEL: func.func @exclusive
  func.func @exclusive(%p: !sec.place<!sec.hw_instance<"device::Control">, "rw">) {
    // CHECK: sec.ref.borrow_mut
    %ref = "sec.ref.borrow_mut"(%p) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.hw_instance<"device::Control">, "rw">) -> !sec.ref_mut<!sec.hw_instance<"device::Control">>
    // CHECK: sec.ref.move
    %next = "sec.ref.move"(%ref) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<!sec.hw_instance<"device::Control">>) -> !sec.ref_mut<!sec.hw_instance<"device::Control">>
    // CHECK: sec.place.deref
    %back = "sec.place.deref"(%next) : (!sec.ref_mut<!sec.hw_instance<"device::Control">>) -> !sec.place<!sec.hw_instance<"device::Control">, "rw">
    return
  }
  // CHECK-LABEL: func.func @detached
  func.func @detached(%p: !sec.place<!sec.register<"device::Control">, "ro">) -> !sec.register<"device::Control"> {
    // CHECK: sec.place.read
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.register<"device::Control">, "ro">) -> !sec.register<"device::Control">
    return %copy : !sec.register<"device::Control">
  }
  // CHECK-LABEL: func.func @shared_holder
  func.func @shared_holder(%p: !sec.storage<!sec.ref<!sec.hw_instance<"device::Control">>>) -> !sec.ref<!sec.hw_instance<"device::Control">> {
    // CHECK: sec.storage.load
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.ref<!sec.hw_instance<"device::Control">>>) -> !sec.ref<!sec.hw_instance<"device::Control">>
    return %copy : !sec.ref<!sec.hw_instance<"device::Control">>
  }
  // CHECK-LABEL: func.func @shared_array
  func.func @shared_array(%p: !sec.place<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "2">, "ro">) -> !sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "2"> {
    // CHECK: sec.place.read
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "2">, "ro">) -> !sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "2">
    return %copy : !sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "2">
  }
  // CHECK-LABEL: func.func @empty_shared_holders
  func.func @empty_shared_holders(%p: !sec.place<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">, "ro">) -> !sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0"> {
    // CHECK: sec.place.read
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">, "ro">) -> !sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">
    return %copy : !sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">
  }

  // CHECK-LABEL: func.func @nested_empty_shared_holders
  func.func @nested_empty_shared_holders(%p: !sec.place<!sec.array<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">, "3">, "ro">) -> !sec.array<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">, "3"> {
    // CHECK: sec.place.read
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.array<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">, "3">, "ro">) -> !sec.array<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">, "3">
    return %copy : !sec.array<!sec.array<!sec.ref<!sec.hw_instance<"device::Control">>, "0">, "3">
  }

}
