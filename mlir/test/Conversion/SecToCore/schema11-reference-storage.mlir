// RUN: sec-mlir-opt %s --sec-lower-trivial-core | FileCheck %s --check-prefix=TRIVIAL
// RUN: sec-mlir-opt %s --sec-lower-scalar-core --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | FileCheck %s --check-prefix=SCALAR

// P15-59: storage that takes part in Place/reference semantics stays
// high-level through P5 trivial-core lowering and the P6 scalar pipeline,
// while ordinary trivial storage keeps its P5 MemRef lowering
// (sec_mlir_lowering_v11.md sections 3 and 18,
// sec-mlir-dialect_package15.md sections 66-67).
module attributes {
  sec.dialect_version = 11 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "reference-storage",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little",
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>
} {
  // TRIVIAL-LABEL: func.func @storages
  // SCALAR-LABEL: func.func @storages
  func.func @storages() -> i32 attributes {sec.extern = false} {
    %one = "sec.const.int"() <{value = "1"}> : () -> i32
    %flag = "sec.const.bool"() <{value = true}> : () -> i1

    // Ordinary storage keeps its P5 lowering.
    // TRIVIAL: memref.alloca() {{.*}}sec.storage_id = 1 : i32{{.*}} : memref<i32>
    // SCALAR: memref.alloca() {{.*}}sec.storage_id = 1 : i32{{.*}} : memref<i32>
    %plain = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "plain", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<i32>
    "sec.storage.init"(%plain, %one) : (!sec.storage<i32>, i32) -> ()

    // Address-taken storage stays semantic storage.
    // TRIVIAL: "sec.storage.declare"() {sec.address_taken = true{{.*}}sec.storage_id = 2 : i32} : () -> !sec.storage<i32>
    // TRIVIAL: "sec.storage.init"({{.*}}) : (!sec.storage<i32>, i32) -> ()
    // SCALAR: "sec.storage.declare"() {sec.address_taken = true{{.*}}sec.storage_id = 2 : i32} : () -> !sec.storage<i32>
    // SCALAR: "sec.place.storage"({{.*}}) {{.*}}: (!sec.storage<i32>) -> !sec.place<i32, "rw">
    %taken = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "taken", sec.storage_class = "local-automatic", sec.storage_id = 2 : i32, sec.address_taken = true} : () -> !sec.storage<i32>
    "sec.storage.init"(%taken, %one) : (!sec.storage<i32>, i32) -> ()
    %place = "sec.place.storage"(%taken) <{place_root_id = 1 : i32, storage_identity = 2 : i32, address_space = "ordinary"}> : (!sec.storage<i32>) -> !sec.place<i32, "rw">
    %ref = "sec.ref.borrow_mut"(%place) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<i32, "rw">) -> !sec.ref_mut<i32>

    // A reference call keeps its argument actions and facts.
    // TRIVIAL: "sec.call.direct"({{.*}}) <{callee = @bump}> {sec.argument_actions = ["mutable-reference"]}
    // SCALAR: "sec.call.direct"({{.*}}) <{callee = @bump}> {sec.argument_actions = ["mutable-reference"]}
    "sec.call.direct"(%ref) <{callee = @bump}> {sec.argument_actions = ["mutable-reference"]} : (!sec.ref_mut<i32>) -> ()
    "sec.ref.end_borrow"(%ref) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref_mut<i32>) -> ()

    // Reference-dependent bool storage does not become memref<i8> in P6.
    // SCALAR: "sec.storage.declare"() {sec.address_taken = true{{.*}}sec.storage_id = 3 : i32} : () -> !sec.storage<i1>
    // SCALAR: "sec.place.storage"({{.*}}) {{.*}}: (!sec.storage<i1>) -> !sec.place<i1, "ro">
    %switch = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "switch", sec.storage_class = "local-automatic", sec.storage_id = 3 : i32, sec.address_taken = true} : () -> !sec.storage<i1>
    "sec.storage.init"(%switch, %flag) : (!sec.storage<i1>, i1) -> ()
    %switchPlace = "sec.place.storage"(%switch) <{place_root_id = 2 : i32, storage_identity = 3 : i32, address_space = "ordinary"}> : (!sec.storage<i1>) -> !sec.place<i1, "ro">
    %on = "sec.place.read"(%switchPlace) <{action = "copy-trivial"}> : (!sec.place<i1, "ro">) -> i1

    // Ordinary bool storage keeps its P6 memref<i8> lowering.
    // SCALAR: memref.alloca() {{.*}}sec.storage_id = 4 : i32{{.*}} : memref<i8>
    %plainFlag = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "plainFlag", sec.storage_class = "local-automatic", sec.storage_id = 4 : i32} : () -> !sec.storage<i1>
    "sec.storage.init"(%plainFlag, %on) : (!sec.storage<i1>, i1) -> ()

    // An ordinary direct call still lowers to func.call.
    // TRIVIAL: call @value()
    %value = "sec.call.direct"() <{callee = @value}> {sec.argument_actions = []} : () -> i32
    %read = "sec.storage.load"(%plain) : (!sec.storage<i32>) -> i32
    return %read : i32
  }

  func.func private @bump(!sec.ref_mut<i32> {sec.reference = {borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}) attributes {sec.extern = false}
  func.func private @value() -> i32 attributes {sec.extern = false}
}
