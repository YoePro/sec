// RUN: sec-mlir-opt %s -o /dev/null

// MD-140: the elimination forms verify with their proof provenance.
module attributes {sec.dialect_version = 15 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arenas", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @bounded(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "proven-bounded", epoch_width = 16 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", epoch_proof = "bound:domain#5<=3", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
  func.func @eliminated(%p: !sec.place<!sec.arena, "rw">) {
    "sec.arena.reset"(%p) <{epoch_advance = "metadata-eliminated", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion", epoch_proof = "no-runtime-handle", state_before = 2 : i32, state_after = 3 : i32, epoch_before = 7 : i32, epoch_after = 8 : i32, backing_kind = "borrowed", growth_kind = "fixed", retained_borrow = 42 : i32, dependency_proof = "no-live-dependency", domain = 5 : i32, transition = "advance-epoch", origin = "Arena", relation = "Borrowed", authority = "ExternalOwner", stability = "Stable", space = "ordinary"}> : (!sec.place<!sec.arena, "rw">) -> ()
    return
  }
}
