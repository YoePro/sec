// RUN: not sec-mlir-opt %s -split-input-file -o /dev/null 2>&1 | FileCheck %s

// MD-140 (sec_mlir_dialect_v15.md section 11): sec.arena.reset carries one
// epoch-advance form, a resolved logical width and the registered
// panic.arena-epoch-exhaustion reason; eliminations carry proof provenance.

// CHECK: epoch_advance must be checked, proven-bounded or metadata-eliminated
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @unknown_form(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "rekey", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
}

// -----

// CHECK: checked epoch advance carries no elimination proof
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @checked_proof(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "checked", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", epoch_proof = "assumed", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
}

// -----

// CHECK: requires proof provenance (epoch_proof)
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bounded_no_proof(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "proven-bounded", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
}

// -----

// CHECK: epoch_width must be a resolved logical width
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @odd_width(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "checked", epoch_width = 12 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
}

// -----

// CHECK: exhaustion_reason must be the registered panic.arena-epoch-exhaustion
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @wrong_reason(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "checked", epoch_width = 64 : i32, exhaustion_reason = "InvalidReferenceGeneration", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
}
