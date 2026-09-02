// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module {
  func.func @bad_default() -> i32 {
    // expected-error@+1 {{result must have !sec.array type}}
    %bad = "sec.array.default"() : () -> i32
    return %bad : i32
  }
}

// -----

module {
  func.func @bad_len(%array: !sec.array<i32, "2">) -> si64 {
    // expected-error@+1 {{result must be !sec.uint or resolved unsigned pointer-width integer}}
    %bad = "sec.array.len"(%array) : (!sec.array<i32, "2">) -> si64
    return %bad : si64
  }
}

// -----

module {
  func.func @bad_signedness(%array: !sec.array<i32, "2">, %index: ui128) -> i1 {
    // expected-error@+1 {{index_signed must match the index semantic type}}
    %bad = "sec.array.index_in_bounds"(%array, %index) <{index_signed = true}> : (!sec.array<i32, "2">, ui128) -> i1
    return %bad : i1
  }
}

// -----

module {
  func.func @bad_extract_result(%array: !sec.array<i32, "2">, %index: si32) -> i64 {
    // expected-error@+1 {{result type must match the array element type}}
    %bad = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.array<i32, "2">, si32) -> i64
    return %bad : i64
  }
}

// -----

module {
  func.func @bad_extract_proof(%array: !sec.array<i32, "2">, %index: si32) -> i32 {
    // expected-error@+1 {{runtime-check bounds proof must be guarded}}
    %bad = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "constant"}> : (!sec.array<i32, "2">, si32) -> i32
    return %bad : i32
  }
}

// -----

module {
  func.func @bad_extract_action(%array: !sec.array<i32, "2">, %index: si32) -> i32 {
    // expected-error@+1 {{action must be copy-trivial}}
    %bad = "sec.array.extract"(%array, %index) <{action = "move", bounds_kind = "proven-safe", bounds_proof = "analysis"}> : (!sec.array<i32, "2">, si32) -> i32
    return %bad : i32
  }
}

// -----

module {
  func.func @bad_replace_value(%array: !sec.array<i32, "2">, %index: si32, %value: i64) -> !sec.array<i32, "2"> {
    // expected-error@+1 {{new value type must match the array element type}}
    %bad = "sec.array.replace"(%array, %index, %value) <{bounds_kind = "proven-safe", bounds_proof = "branch"}> : (!sec.array<i32, "2">, si32, i64) -> !sec.array<i32, "2">
    return %bad : !sec.array<i32, "2">
  }
}

// -----

module {
  func.func @bad_replace_result(%array: !sec.array<i32, "2">, %index: si32, %value: i32) -> !sec.array<i32, "3"> {
    // expected-error@+1 {{result type must exactly match the source array type}}
    %bad = "sec.array.replace"(%array, %index, %value) <{bounds_kind = "proven-safe", bounds_proof = "branch"}> : (!sec.array<i32, "2">, si32, i32) -> !sec.array<i32, "3">
    return %bad : !sec.array<i32, "3">
  }
}

// -----

module {
  func.func @bad_failure() {
    // expected-error@+1 {{operation must be fixed-array-index}}
    "sec.fail.bounds"() {operation = "array-index"} : () -> ()
  }
}
