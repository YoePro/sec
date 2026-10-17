// RUN: split-file %s %t
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | FileCheck %s --check-prefix=P64
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics | FileCheck %s --check-prefix=P32
// RUN: not sec-mlir-opt %t/64.mlir --sec-lower-integer-core 2>&1 | FileCheck %s --check-prefix=P8
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout | not sec-mlir-opt --sec-lower-checked-integers 2>&1 | FileCheck %s --check-prefix=P8

// P15-60: P6 target scalar resolution recursively converts only the referents
// under !sec.place, !sec.ref and !sec.ref_mut on 32- and 64-bit plans and keeps
// the wrappers, authority, operations and reference metadata. P8 signless
// normalization neither recurses through the wrappers nor lowers P15
// operations: a module with Place/reference semantics is rejected before any
// rewrite (sec_mlir_dialect_v11.md sections 32-33, sec_mlir_lowering_v11.md
// sections 19-20; MD-087).

// P64-LABEL: func.func @resolve
// P64-SAME: %arg0: !sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si64, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui64, "2">, tags = []>]>> {sec.reference = {address_stability = "Stable", borrow_id = 1 : i32
// P64-SAME: %arg1: si64
// P64: "sec.place.deref"(%arg0) : (!sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si64, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui64, "2">, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si64, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui64, "2">, tags = []>]>, "rw">
// P64: "sec.place.field"({{.*}}) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si64, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui64, "2">, tags = []>]>, "rw">) -> !sec.place<si64, "rw">
// P64: "sec.place.array_index_in_bounds"({{.*}}) <{index_signed = true}> : (!sec.place<!sec.array<ui64, "2">, "rw">, si64) -> i1
// P64: "sec.place.array_element"({{.*}}) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<ui64, "2">, "rw">, si64) -> !sec.place<ui64, "rw">
// P64: "sec.ref.reborrow_shared"({{.*}}) <{address_stability = "Stable", borrow_id = 2 : i32, lifetime_id = 2 : i32, provenance = "parameter", source_borrow = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow"}> : (!sec.place<ui64, "rw">) -> !sec.ref<ui64>
// P64: "sec.ref.end_borrow"({{.*}}) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<ui64>) -> ()
// P64: "sec.place.read"({{.*}}) <{action = "copy-trivial"}> : (!sec.place<si64, "rw">) -> si64
// P64: "sec.place.write"({{.*}}) : (!sec.place<si64, "rw">, si64) -> ()
// P64-NOT: !sec.int
// P64-NOT: !sec.uint

// P32-LABEL: func.func @resolve
// P32-SAME: %arg0: !sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si32, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui32, "2">, tags = []>]>> {sec.reference = {address_stability = "Stable", borrow_id = 1 : i32
// P32-SAME: %arg1: si32
// P32: "sec.place.deref"(%arg0) : (!sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si32, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui32, "2">, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si32, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui32, "2">, tags = []>]>, "rw">
// P32: "sec.place.field"({{.*}}) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = si32, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<ui32, "2">, tags = []>]>, "rw">) -> !sec.place<si32, "rw">
// P32: "sec.place.array_index_in_bounds"({{.*}}) <{index_signed = true}> : (!sec.place<!sec.array<ui32, "2">, "rw">, si32) -> i1
// P32: "sec.place.array_element"({{.*}}) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<ui32, "2">, "rw">, si32) -> !sec.place<ui32, "rw">
// P32: "sec.ref.reborrow_shared"({{.*}}) <{address_stability = "Stable", borrow_id = 2 : i32, lifetime_id = 2 : i32, provenance = "parameter", source_borrow = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow"}> : (!sec.place<ui32, "rw">) -> !sec.ref<ui32>
// P32: "sec.ref.end_borrow"({{.*}}) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<ui32>) -> ()
// P32: "sec.place.read"({{.*}}) <{action = "copy-trivial"}> : (!sec.place<si32, "rw">) -> si32
// P32: "sec.place.write"({{.*}}) : (!sec.place<si32, "rw">, si32) -> ()
// P32-NOT: !sec.int
// P32-NOT: !sec.uint

// P8: Place/reference semantics must be discharged by reference representation lowering before checked integer lowering

//--- 64.mlir
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "resolve-64", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @resolve(%cell: !sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: !sec.int) -> !sec.int attributes {sec.extern = false} {
    %whole = "sec.place.deref"(%cell) : (!sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>, "rw">
    %count = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>, "rw">) -> !sec.place<!sec.int, "rw">
    %sizes = "sec.place.field"(%whole) <{field = 1 : i32}> : (!sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>, "rw">) -> !sec.place<!sec.array<!sec.uint, "2">, "rw">
    %in = "sec.place.array_index_in_bounds"(%sizes, %index) <{index_signed = true}> : (!sec.place<!sec.array<!sec.uint, "2">, "rw">, !sec.int) -> i1
    cf.cond_br %in, ^ok, ^bad
  ^ok:
    %size = "sec.place.array_element"(%sizes, %index) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.uint, "2">, "rw">, !sec.int) -> !sec.place<!sec.uint, "rw">
    %shared = "sec.ref.reborrow_shared"(%size) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.uint, "rw">) -> !sec.ref<!sec.uint>
    "sec.ref.end_borrow"(%shared) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<!sec.uint>) -> ()
    %value = "sec.place.read"(%count) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
    "sec.place.write"(%count, %index) : (!sec.place<!sec.int, "rw">, !sec.int) -> ()
    return %value : !sec.int
  ^bad:
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }
}

//--- 32.mlir
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "resolve-32", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "armv7", sec.target_triple = "armv7-unknown-linux-gnueabihf", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func @resolve(%cell: !sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}}, %index: !sec.int) -> !sec.int attributes {sec.extern = false} {
    %whole = "sec.place.deref"(%cell) : (!sec.ref_mut<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>>) -> !sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>, "rw">
    %count = "sec.place.field"(%whole) <{field = 0 : i32}> : (!sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>, "rw">) -> !sec.place<!sec.int, "rw">
    %sizes = "sec.place.field"(%whole) <{field = 1 : i32}> : (!sec.place<!sec.struct<identity = "main::Cell", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "count", type = !sec.int, tags = []>, #sec.struct_field<ordinal = 1, name = "sizes", type = !sec.array<!sec.uint, "2">, tags = []>]>, "rw">) -> !sec.place<!sec.array<!sec.uint, "2">, "rw">
    %in = "sec.place.array_index_in_bounds"(%sizes, %index) <{index_signed = true}> : (!sec.place<!sec.array<!sec.uint, "2">, "rw">, !sec.int) -> i1
    cf.cond_br %in, ^ok, ^bad
  ^ok:
    %size = "sec.place.array_element"(%sizes, %index) <{bounds_kind = "runtime-check", bounds_proof = "guarded"}> : (!sec.place<!sec.array<!sec.uint, "2">, "rw">, !sec.int) -> !sec.place<!sec.uint, "rw">
    %shared = "sec.ref.reborrow_shared"(%size) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter", source_borrow = 1 : i32}> : (!sec.place<!sec.uint, "rw">) -> !sec.ref<!sec.uint>
    "sec.ref.end_borrow"(%shared) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<!sec.uint>) -> ()
    %value = "sec.place.read"(%count) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
    "sec.place.write"(%count, %index) : (!sec.place<!sec.int, "rw">, !sec.int) -> ()
    return %value : !sec.int
  ^bad:
    "sec.fail.bounds"() {operation = "fixed-array-index"} : () -> ()
  }
}
