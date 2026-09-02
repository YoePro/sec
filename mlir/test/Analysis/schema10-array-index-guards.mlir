// RUN: sec-mlir-opt --sec-verify-array-index-guards %s -o /dev/null

module {
  func.func @runtime_extract(%array: !sec.array<i32, "4">, %index: si128) -> i32 {
    %inside = "sec.array.index_in_bounds"(%array, %index) <{index_signed = true}> : (!sec.array<i32, "4">, si128) -> i1
    cf.cond_br %inside, ^success, ^failure
  ^success:
    cf.br ^project
  ^project:
    %value = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, si128) -> i32
    return %value : i32
  ^failure:
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }

  func.func @runtime_replace(%array: !sec.array<i32, "4">, %index: ui128, %new_value: i32) -> !sec.array<i32, "4"> {
    %inside = "sec.array.index_in_bounds"(%array, %index) <{index_signed = false}> : (!sec.array<i32, "4">, ui128) -> i1
    cf.cond_br %inside, ^success, ^failure
  ^success:
    %value = "sec.array.replace"(%array, %index, %new_value) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.array<i32, "4">, ui128, i32) -> !sec.array<i32, "4">
    return %value : !sec.array<i32, "4">
  ^failure:
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }

  func.func @proven(%array: !sec.array<i32, "4">, %index: si32) -> i32 {
    %constant = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "constant"}> : (!sec.array<i32, "4">, si32) -> i32
    %range = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<i32, "4">, si32) -> i32
    %branch = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "branch"}> : (!sec.array<i32, "4">, si32) -> i32
    %contract = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "contract"}> : (!sec.array<i32, "4">, si32) -> i32
    %analysis = "sec.array.extract"(%array, %index) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "analysis"}> : (!sec.array<i32, "4">, si32) -> i32
    return %analysis : i32
  }
}
