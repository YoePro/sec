// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s
// MD-077 § 19(5), semantic_ir.md § 21(11): binding roots round-trip unchanged.
module attributes {
  sec.registers = [#sec.register_def<identity = "device::Control", width = 32, order = "lsb-first", fields = [#sec.register_field<ordinal = 0, name = "Raw", offset = 0, width = 32, type = ui32, access = "read-write", shadow = false, reserved = false>]>],
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "md077-hw-bind",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // CHECK: "sec.hw.binding"() <{address = "1073741840", alias = false, alignment = "4", domain = "cpu-physical", endpoint_id = "project:p:board::ControlPort", endpoint_kind = "MemoryMapped", hardware_binding_id = 1 : i32, hardware_resource_id = "project:p:board::ControlState", instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, read_widths = [32], region_id = "project:p:board::Space", size = "4", sym_name = "device::control", write_widths = [32]}> : () -> ()
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
  // A domain address beyond 64 bits stays exact.
  // CHECK: address = "340282366920938463463374607431768211455"
  "sec.hw.binding"() <{sym_name = "device::wide", hardware_binding_id = 2 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = false, alias = true, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "340282366920938463463374607431768211455", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}
