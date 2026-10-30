// RUN: sec-mlir-opt %s --split-input-file --verify-diagnostics --sec-check-lowering-support
// Payloads without a selected representation stop at the track entry by name.

!U = !sec.union<identity = "main::Named", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Text", kind = "single", payload = !sec.string, fields = []>]>

func.func @string_payload(%value: !sec.string) -> !U {
  // expected-error @+1 {{does not lower 'sec.union.construct' yet}}
  %made = "sec.union.construct"(%value) <{fieldNames = [], payloadActions = ["copy-trivial"], variant = 0 : i32}> : (!sec.string) -> !U
  return %made : !U
}

// -----

!R = !sec.result<!sec.string, i8>

func.func @storage_of_string_result(%value: !R) {
  // expected-error @+1 {{does not lower 'sec.storage.declare' yet}}
  %slot = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "slot", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!R>
  // expected-error @+1 {{does not lower 'sec.storage.init' yet}}
  "sec.storage.init"(%slot, %value) : (!sec.storage<!R>, !R) -> ()
  return
}
