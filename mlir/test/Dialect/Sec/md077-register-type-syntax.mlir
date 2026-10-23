// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// RUN: sed 's/sec.dialect_version = 11/sec.dialect_version = 12/' %s | sec-mlir-opt | sec-mlir-opt | FileCheck %s

// MD-077 section 19(1)-(2): type spelling only. Register-table resolution
// and hardware operation admission are not implemented by this fixture.
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "register-types",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK: func.func private @value(!sec.register<"device::Control">) -> !sec.register<"device::Control">
  func.func private @value(!sec.register<"device::Control">) -> !sec.register<"device::Control">
  // CHECK: func.func private @instance(!sec.hw_instance<"device::Control">) -> !sec.hw_instance<"device::Control">
  func.func private @instance(!sec.hw_instance<"device::Control">) -> !sec.hw_instance<"device::Control">
  // CHECK: func.func private @shared(!sec.ref<!sec.hw_instance<"device::Control">>) -> !sec.ref<!sec.hw_instance<"device::Control">>
  func.func private @shared(!sec.ref<!sec.hw_instance<"device::Control">>) -> !sec.ref<!sec.hw_instance<"device::Control">>
  // CHECK: func.func private @exclusive(!sec.ref_mut<!sec.hw_instance<"device::Control">>) -> !sec.ref_mut<!sec.hw_instance<"device::Control">>
  func.func private @exclusive(!sec.ref_mut<!sec.hw_instance<"device::Control">>) -> !sec.ref_mut<!sec.hw_instance<"device::Control">>
  // CHECK: func.func private @detached_reference(!sec.ref<!sec.register<"device::Control">>) -> !sec.ref<!sec.register<"device::Control">>
  func.func private @detached_reference(!sec.ref<!sec.register<"device::Control">>) -> !sec.ref<!sec.register<"device::Control">>
}
