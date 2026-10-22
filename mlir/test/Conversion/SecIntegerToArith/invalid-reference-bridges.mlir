// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics --sec-lower-checked-integers

// MD-087 §§ 4(6), 5(4): the only representation bridge checked integer
// lowering leaves at a Place/reference boundary is an exact-width
// semantic <-> signless integer cast; any other surviving cast is rejected.
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "bridges", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @widening_bridge(%place: !sec.place<si64, "rw">, %value: si32) attributes {sec.extern = false} {
    // expected-error@+1 {{checked integer lowering left a bridge that is not an exact-width semantic/signless integer representation transition}}
    %wide = builtin.unrealized_conversion_cast %value : si32 to si64
    "sec.place.write"(%place, %wide) : (!sec.place<si64, "rw">, si64) -> ()
    return
  }
}
