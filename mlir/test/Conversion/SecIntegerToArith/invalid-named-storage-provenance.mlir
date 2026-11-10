// RUN: not sec-mlir-opt --sec-lower-integer-core %s 2>&1 | FileCheck %s
// A nominal wrapper must not excuse metadata that contradicts its carrier.
// Rules: rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md section 5(3)-(4).
!meters = !sec.named<"main::Meters", si64>
module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wrong_provenance() {
    %slot = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "uint64", sec.source_name = "meters", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!meters>
    return
  }
}
// CHECK: Sec-origin named integer storage has sec.scalar_kind 'uint64' incompatible with 'si64'
