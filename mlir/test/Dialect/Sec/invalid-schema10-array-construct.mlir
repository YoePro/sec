// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module {
  // expected-error@+1 {{segment operands, kinds, lengths, and actions must have equal counts}}
  %bad = "sec.array.construct"() <{segment_actions = [], segment_kinds = ["element"], segment_lengths = []}> : () -> !sec.array<i32, "0">
}

// -----

module {
  %value = arith.constant 1 : i32
  // expected-error@+1 {{element segment action must be construct-direct}}
  %bad = "sec.array.construct"(%value) <{segment_actions = ["copy-trivial"], segment_kinds = ["element"], segment_lengths = ["1"]}> : (i32) -> !sec.array<i32, "1">
}

// -----

module {
  %value = arith.constant 1 : i32
  // expected-error@+1 {{element segment length must be 1}}
  %bad = "sec.array.construct"(%value) <{segment_actions = ["construct-direct"], segment_kinds = ["element"], segment_lengths = ["2"]}> : (i32) -> !sec.array<i32, "2">
}

// -----

module {
  %value = arith.constant 1 : i32
  // expected-error@+1 {{element segment type must match the result element type}}
  %bad = "sec.array.construct"(%value) <{segment_actions = ["construct-direct"], segment_kinds = ["element"], segment_lengths = ["1"]}> : (i32) -> !sec.array<i64, "1">
}

// -----

module {
  func.func @bad(%spread: !sec.array<i32, "2">) {
    // expected-error@+1 {{spread segment length must match its operand array length}}
    %bad = "sec.array.construct"(%spread) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["3"]}> : (!sec.array<i32, "2">) -> !sec.array<i32, "3">
    return
  }
}

// -----

module {
  func.func @bad(%spread: !sec.array<i32, "2">) {
    // expected-error@+1 {{spread segment action must be copy-trivial}}
    %bad = "sec.array.construct"(%spread) <{segment_actions = ["construct-direct"], segment_kinds = ["spread"], segment_lengths = ["2"]}> : (!sec.array<i32, "2">) -> !sec.array<i32, "2">
    return
  }
}

// -----

module {
  %value = arith.constant 1 : i32
  // expected-error@+1 {{spread segment must be an array of the result element type}}
  %bad = "sec.array.construct"(%value) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["1"]}> : (i32) -> !sec.array<i32, "1">
}

// -----

module {
  %value = arith.constant 1 : i32
  // expected-error@+1 {{array segment kind must be element or spread}}
  %bad = "sec.array.construct"(%value) <{segment_actions = ["construct-direct"], segment_kinds = ["other"], segment_lengths = ["1"]}> : (i32) -> !sec.array<i32, "1">
}

// -----

module {
  %first = arith.constant 1 : i32
  %second = arith.constant 2 : i32
  // expected-error@+1 {{exact segment length sum must match the result array length}}
  %bad = "sec.array.construct"(%first, %second) <{segment_actions = ["construct-direct", "construct-direct"], segment_kinds = ["element", "element"], segment_lengths = ["1", "1"]}> : (i32, i32) -> !sec.array<i32, "3">
}

// -----

module {
  %value = arith.constant 1 : i32
  // expected-error@+1 {{array segment lengths must be canonical unsigned decimal}}
  %bad = "sec.array.construct"(%value) <{segment_actions = ["construct-direct"], segment_kinds = ["element"], segment_lengths = ["01"]}> : (i32) -> !sec.array<i32, "1">
}
