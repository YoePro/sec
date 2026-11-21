// RUN: sec-mlir-opt %s --split-input-file --verify-diagnostics --sec-lower-unit-dimensions
// A dimension reaching a consumer without a physical representation, here
// storage, is rejected by name instead of being erased (MD-062 § 7(12)).

func.func @stored(%d: !sec.unit_dimension) {
  // expected-error @+1 {{does not lower 'sec.storage.declare' over UnitDimension values yet}}
  %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "kept", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!sec.unit_dimension>
  return
}
