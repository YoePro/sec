// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-077 § 19(5): a binding root keeps every resolved fact or fails verification.
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
  // expected-error@+1 {{instance_type must be !sec.hw_instance<T>}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.register<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}

// -----

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
  // expected-error@+1 {{bound register type has no definition in sec.registers}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Missing">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}

// -----

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
  // expected-error@+1 {{requires resolved endpoint, resource, region and domain identities}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}

// -----

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
  // expected-error@+1 {{endpoint kind 'PortMapped' has no admitted binding schema}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "PortMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}

// -----

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
  // expected-error@+1 {{requires a decimal address and a positive decimal size and alignment}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "0", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}

// -----

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
  // expected-error@+1 {{requires a decimal address and a positive decimal size and alignment}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "0x40000010", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}

// -----

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
  // expected-error@+1 {{transfer width 64 is not positive or exceeds the endpoint extent}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [64]}> : () -> ()
}

// -----

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
  // expected-error@+1 {{hardware_binding_id must be positive}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 0 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}

// -----

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
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
  // expected-error@+1 {{duplicate hardware_binding_id 1}}
  "sec.hw.binding"() <{sym_name = "device::other", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}
