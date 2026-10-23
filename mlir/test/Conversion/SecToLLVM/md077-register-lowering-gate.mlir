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
