// RUN: sec-mlir-opt %s | sec-mlir-opt -o /dev/null
// Dynamic proof data may be omitted; provided data retains its string type.
module {
  func.func private @dynamic(!sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "parameter"}}) -> !sec.ref<si32> attributes {sec.return_origins = ["param#0"]}
  func.func private @proof(!sec.ref<si32> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "parameter", validity_proof = "resolved-analysis"}})
  func.func private @slice(!sec.slice<si32>) -> !sec.slice<si32> attributes {sec.return_origins = ["param#0"]}
}
