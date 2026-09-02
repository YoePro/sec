// RUN: sec-mlir-opt --sec-verify-array-index-guards %s -split-input-file -verify-diagnostics -o /dev/null

module {
  func.func @missing_guard(%array: !sec.array<i32, "4">, %index: si32) -> i32 {
    // expected-error@+1 {{runtime-check array operation requires a dominating true edge from sec.array.index_in_bounds on the same array and index SSA values}}
    %value = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, si32) -> i32
    return %value : i32
  }
}

// -----

module {
  func.func @replacement_without_guard(%array: !sec.array<i32, "4">, %index: si32, %new_value: i32) -> !sec.array<i32, "4"> {
    // expected-error@+1 {{runtime-check array operation requires a dominating true edge from sec.array.index_in_bounds on the same array and index SSA values}}
    %value = "sec.array.replace"(%array, %index, %new_value) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, si32, i32) -> !sec.array<i32, "4">
    return %value : !sec.array<i32, "4">
  }
}

// -----

module {
  func.func @wrong_index(%array: !sec.array<i32, "4">, %tested: si32, %used: si32) -> i32 {
    %inside = "sec.array.index_in_bounds"(%array, %tested) <{index_signed = true}> : (!sec.array<i32, "4">, si32) -> i1
    cf.cond_br %inside, ^success, ^failure
  ^success:
    // expected-error@+1 {{runtime-check array operation requires a dominating true edge from sec.array.index_in_bounds on the same array and index SSA values}}
    %value = "sec.array.extract"(%array, %used) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, si32) -> i32
    return %value : i32
  ^failure:
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }
}

// -----

module {
  func.func @wrong_array(%tested: !sec.array<i32, "4">, %used: !sec.array<i32, "4">, %index: si32) -> i32 {
    %inside = "sec.array.index_in_bounds"(%tested, %index) <{index_signed = true}> : (!sec.array<i32, "4">, si32) -> i1
    cf.cond_br %inside, ^success, ^failure
  ^success:
    // expected-error@+1 {{runtime-check array operation requires a dominating true edge from sec.array.index_in_bounds on the same array and index SSA values}}
    %value = "sec.array.extract"(%used, %index) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, si32) -> i32
    return %value : i32
  ^failure:
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }
}

// -----

module {
  func.func @false_edge(%array: !sec.array<i32, "4">, %index: si32) -> i32 {
    %inside = "sec.array.index_in_bounds"(%array, %index) <{index_signed = true}> : (!sec.array<i32, "4">, si32) -> i1
    cf.cond_br %inside, ^success, ^failure
  ^success:
    %zero = arith.constant 0 : i32
    return %zero : i32
  ^failure:
    // expected-error@+1 {{runtime-check array operation requires a dominating true edge from sec.array.index_in_bounds on the same array and index SSA values}}
    %value = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, si32) -> i32
    return %value : i32
  }
}

// -----

module {
  func.func @invalid_provenance(%array: !sec.array<i32, "4">, %index: si32) -> i32 {
    // expected-error@+1 {{proven-safe bounds proof must be constant, range, branch, contract, or analysis}}
    %value = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = ""}> : (!sec.array<i32, "4">, si32) -> i32
    return %value : i32
  }
}

// -----

module {
  func.func @rejoined_edges(%array: !sec.array<i32, "4">, %index: si32) -> i32 {
    %inside = "sec.array.index_in_bounds"(%array, %index) <{index_signed = true}> : (!sec.array<i32, "4">, si32) -> i1
    cf.cond_br %inside, ^success, ^failure
  ^success:
    cf.br ^merge
  ^failure:
    cf.br ^merge
  ^merge:
    // expected-error@+1 {{runtime-check array operation requires a dominating true edge from sec.array.index_in_bounds on the same array and index SSA values}}
    %value = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, si32) -> i32
    return %value : i32
  }
}
