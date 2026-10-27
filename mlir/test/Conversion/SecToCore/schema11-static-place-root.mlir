// RUN: sec-mlir-opt %s --sec-lower-trivial-core --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | FileCheck %s --check-prefix=TRIVIAL
// RUN: sec-mlir-opt %s --sec-lower-scalar-core --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | FileCheck %s --check-prefix=SCALAR
// RUN: sec-mlir-opt %s --sec-lower-scalar-core --sec-lower-integer-core --sec-verify-places --sec-verify-borrow-semantics | FileCheck %s --check-prefix=INTEGER
// RUN: not sec-mlir-opt %s --sec-lower-scalar-core --sec-lower-integer-core --sec-lower-static-storage 2>&1 | FileCheck %s --check-prefix=STATIC
// RUN: not sec-mlir-opt %s --sec-lower-to-llvm=entry= 2>&1 | FileCheck %s --check-prefix=LLVM

// MD-116 §§ 3(3), 7(5)-(6): a reference-dependent static keeps its declaration,
// identity, address-taken marker and Place root through P5, P6 and P8. P6
// resolves a target-sized declaration and its Place roots together to the
// plan's width. Physical static lowering rejects the
// static instead of erasing the Place.

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  "sec.static.global"() <{initial = "10", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 1 : i32, sec.address_taken = true, sec.source_name = "main::Count"} : () -> ()
  "sec.static.global"() <{initial = "0", is_mutable = true, sym_name = "sec_static_2", type = !sec.int}> {sec.static_id = 2 : i32, sec.address_taken = true, sec.source_name = "main::Total"} : () -> ()
  func.func @sec_fn_0() -> (!sec.ref<si32>) attributes {sec.function_id = "main::Count()", sec.source_name = "Count", sec.extern = false, sec.unsafe = false, sec.parameter_names = [], sec.return_origins = ["static@1"]} {
    %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    %r = "sec.ref.borrow_shared"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<si32, "ro">) -> !sec.ref<si32>
    return %r : !sec.ref<si32>
  }
  func.func @sec_fn_1() -> (!sec.ref_mut<!sec.int>) attributes {sec.function_id = "main::Total()", sec.source_name = "Total", sec.extern = false, sec.unsafe = false, sec.parameter_names = [], sec.return_origins = ["static@2"]} {
    %root = "sec.place.static"() <{global = @sec_static_2, place_root_id = 2 : i32, storage_identity = 2 : i32, address_space = "ordinary"}> : () -> !sec.place<!sec.int, "rw">
    %r = "sec.ref.borrow_mut"(%root) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "static-storage", address_stability = "Fixed", provenance = "static"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
    return %r : !sec.ref_mut<!sec.int>
  }
}

// TRIVIAL: "sec.static.global"() {{.*}}type = si32}> {sec.address_taken = true, {{.*}}sec.static_id = 1 : i32}
// TRIVIAL: "sec.place.static"() {{.*}}global = @sec_static_1{{.*}} -> !sec.place<si32, "ro">
// TRIVIAL: "sec.place.static"() {{.*}}global = @sec_static_2{{.*}} -> !sec.place<!sec.int, "rw">
// SCALAR: "sec.static.global"() {{.*}}type = si64}> {sec.address_taken = true, {{.*}}sec.static_id = 2 : i32}
// SCALAR: "sec.place.static"() {{.*}}global = @sec_static_1{{.*}} -> !sec.place<si32, "ro">
// SCALAR: "sec.place.static"() {{.*}}global = @sec_static_2{{.*}} -> !sec.place<si64, "rw">
// INTEGER: "sec.place.static"() {{.*}}global = @sec_static_1
// INTEGER: "sec.place.static"() {{.*}}global = @sec_static_2
// STATIC: error: the Sec MLIR -> LLVM track does not lower reference-dependent static storage yet
// LLVM: error: [unsupported-lowering] the Sec MLIR -> LLVM track does not lower 'sec.place.static' yet
