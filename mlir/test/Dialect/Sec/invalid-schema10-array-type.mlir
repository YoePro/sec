// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module {
  // expected-error@+1 {{sec.array length must be canonical unsigned decimal}}
  func.func @empty(%value: !sec.array<i32, "">)
}

// -----

module {
  // expected-error@+1 {{sec.array length must be canonical unsigned decimal}}
  func.func @negative(%value: !sec.array<i32, "-1">)
}

// -----

module {
  // expected-error@+1 {{sec.array length must be canonical unsigned decimal}}
  func.func @plus(%value: !sec.array<i32, "+1">)
}

// -----

module {
  // expected-error@+1 {{sec.array length must not contain unnecessary leading zeroes}}
  func.func @leading_zero(%value: !sec.array<i32, "01">)
}

// -----

module {
  // expected-error@+1 {{sec.array length must not contain unnecessary leading zeroes}}
  func.func @double_zero(%value: !sec.array<i32, "00">)
}

// -----

module {
  // expected-error@+1 {{sec.array length must be canonical unsigned decimal}}
  func.func @whitespace(%value: !sec.array<i32, " 4 ">)
}

// -----

module {
  // expected-error@+1 {{sec.array element type must be a sized semantic value type}}
  func.func @storage_element(%value: !sec.array<!sec.storage<i32>, "1">)
}

// -----

module {
  // expected-error@+1 {{sec.array element type must be a sized semantic value type}}
  func.func @function_element(%value: !sec.array<(i32) -> i32, "1">)
}
