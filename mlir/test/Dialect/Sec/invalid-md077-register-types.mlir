// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module {
  // expected-error@+1 {{sec.register identity must not be empty}}
  func.func private @empty(!sec.register<"">)
}

// -----

module {
  // expected-error@+1 {{sec.hw_instance identity must not be empty}}
  func.func private @empty(!sec.hw_instance<"">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 1}}
module attributes {sec.dialect_version = 1 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 1}}
module attributes {sec.dialect_version = 1 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 1}}
module attributes {sec.dialect_version = 1 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 2}}
module attributes {sec.dialect_version = 2 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 2}}
module attributes {sec.dialect_version = 2 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 2}}
module attributes {sec.dialect_version = 2 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 3}}
module attributes {sec.dialect_version = 3 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 3}}
module attributes {sec.dialect_version = 3 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 3}}
module attributes {sec.dialect_version = 3 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 4}}
module attributes {sec.dialect_version = 4 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 4}}
module attributes {sec.dialect_version = 4 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 4}}
module attributes {sec.dialect_version = 4 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 5}}
module attributes {sec.dialect_version = 5 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 5}}
module attributes {sec.dialect_version = 5 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 5}}
module attributes {sec.dialect_version = 5 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 6}}
module attributes {sec.dialect_version = 6 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 6}}
module attributes {sec.dialect_version = 6 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 6}}
module attributes {sec.dialect_version = 6 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 7}}
module attributes {sec.dialect_version = 7 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 7}}
module attributes {sec.dialect_version = 7 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 7}}
module attributes {sec.dialect_version = 7 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 8}}
module attributes {sec.dialect_version = 8 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 8}}
module attributes {sec.dialect_version = 8 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 8}}
module attributes {sec.dialect_version = 8 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 9}}
module attributes {sec.dialect_version = 9 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 9}}
module attributes {sec.dialect_version = 9 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 9}}
module attributes {sec.dialect_version = 9 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 10}}
module attributes {sec.dialect_version = 10 : i32} {
  func.func private @old(!sec.register<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 10}}
module attributes {sec.dialect_version = 10 : i32} {
  func.func private @old(!sec.hw_instance<"device::Control">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11, module declares schema 10}}
module attributes {sec.dialect_version = 10 : i32} {
  func.func private @old(!sec.array<!sec.register<"device::Control">, "2">)
}

// -----

// expected-error@+1 {{register and hardware-instance types require schema 11}}
module attributes {sec.dialect_version = 10 : i32, sec.test_metadata = {nested = [!sec.hw_instance<"device::Control">]}} {}

// -----

module {
  func.func @distinct(%arg: !sec.hw_instance<"device::Control">) -> !sec.register<"device::Control"> {
    // expected-error@+1 {{type of return operand 0}}
    return %arg : !sec.hw_instance<"device::Control">
  }
}

// -----

module {
  func.func @nominal(%arg: !sec.register<"device::Control">) -> !sec.register<"device::Status"> {
    // expected-error@+1 {{type of return operand 0}}
    return %arg : !sec.register<"device::Control">
  }
}

// -----

module {
  func.func @integer(%arg: i32) -> !sec.register<"device::Control"> {
    // expected-error@+1 {{type of return operand 0}}
    return %arg : i32
  }
}
