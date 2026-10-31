// RUN: sec-mlir-opt %s --sec-check-lowering-support -split-input-file --verify-diagnostics -o /dev/null
// RUN: sec-mlir-opt %s --sec-lower-to-llvm -split-input-file --verify-diagnostics -o /dev/null

// MD-077 section 19: parse/print support selects no physical representation.
// The public pipeline must reject unsupported register identities at entry,
// including declarations that have no Sec operations at all.
module {
  // expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower type '!sec.register<"device::Control">' yet}}
  func.func private @detached(!sec.register<"device::Control">)
}

// -----

module {
  // expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower type '!sec.hw_instance<"device::Control">' yet}}
  func.func private @instance() -> !sec.hw_instance<"device::Control">
}

// -----

module {
  // expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower type '!sec.register<"device::Control">' yet}}
  func.func private @nested(!sec.ref<!sec.array<!sec.register<"device::Control">, "2">>)
}

// -----

// Type-valued metadata must not conceal an unsupported physical dependency.
// expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower type '!sec.hw_instance<"device::Control">' yet}}
module attributes {sec.test_metadata = {nested = [!sec.hw_instance<"device::Control">]}} {}

// -----

module {
  // Unreachable block arguments are still representation obligations.
  // expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower type '!sec.register<"device::Control">' yet}}
  func.func @block_argument() {
    return
  ^unreachable(%value: !sec.register<"device::Control">):
    return
  }
}

// -----

// A register.hw.bind root selects no physical representation either.
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
  // expected-error@+2 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower 'sec.hw.binding' yet}}
  // expected-error@+1 {{[unsupported-lowering] the Sec MLIR -> LLVM track does not lower type '!sec.hw_instance<"device::Control">' yet}}
  "sec.hw.binding"() <{sym_name = "device::control", hardware_binding_id = 1 : i32, instance_type = !sec.hw_instance<"device::Control">, is_mutable = true, alias = false, endpoint_id = "project:p:board::ControlPort", hardware_resource_id = "project:p:board::ControlState", region_id = "project:p:board::Space", endpoint_kind = "MemoryMapped", domain = "cpu-physical", address = "1073741840", size = "4", alignment = "4", read_widths = [32], write_widths = [32]}> : () -> ()
}
