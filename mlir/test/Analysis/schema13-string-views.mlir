// RUN: sec-mlir-opt %s --sec-verify-slice-guards --sec-verify-places --sec-verify-reference-guards --sec-verify-borrow-semantics --sec-verify-ownership | sec-mlir-opt --sec-verify-slice-guards | FileCheck %s
// MD-126: StringView construction, holder reuse of the shared bounded-view
// slice operations, returned views and rune-by-value decoding verify and round-trip.
// Rules: rules/collections/collections.md 7.10; rules/control-flow/flowcontrol_for.md 19;
// rules/compiler/semantic_ir.md 17(9)-(10).
// CHECK-LABEL: func.func @sec_fn_0
// CHECK: "sec.slice.len"({{.*}}) : (!sec.string_view) -> !sec.uint
// CHECK: "sec.string_view.byte_len"
// CHECK-LABEL: func.func @sec_fn_1
// CHECK: "sec.string.rune_len"({{.*}}) : (!sec.place<!sec.string, "ro">) -> !sec.uint
// CHECK: "sec.slice.range_check"
// CHECK: "sec.string.view_borrow"
// CHECK-LABEL: func.func @sec_fn_2
// CHECK: "sec.slice.reborrow_shared"({{.*}}) {{.*}} : (!sec.string_view, !sec.uint, !sec.uint) -> !sec.string_view
// CHECK-LABEL: func.func @sec_fn_3
// CHECK: "sec.string_view.decode_next"({{.*}}) : (!sec.string_view, !sec.uint) -> (i1, !sec.rune, !sec.uint)
// CHECK: "sec.ref.end_borrow"
module attributes {dlti.dl_spec = #dlti.dl_spec<index = 64 : i64>, sec.dialect_version = 13 : i32, sec.module_id = "main", sec.semantic_ir_version = 1 : i32, sec.source_files = ["string-views.sec"], sec.target_abi = "gnu", sec.target_arch = "amd64", sec.target_endianness = "little", sec.target_os = "linux", sec.target_profile = "hosted", sec.target_triple = "x86_64-pc-linux-gnu"} {
  func.func @sec_fn_0(%arg0: !sec.string_view {sec.reference = {address_stability = "Stable", borrow_id = 24 : i32, lifetime_id = 24 : i32, provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}) -> (!sec.uint {sec.scalar_kind = "uint"}) attributes {sec.extern = false, sec.function_id = "main::Count(StringView)", sec.parameter_names = ["view"], sec.source_name = "Count", sec.unsafe = false} {
    %0 = "sec.slice.len"(%arg0) : (!sec.string_view) -> !sec.uint
    %1 = "sec.string_view.byte_len"(%arg0) : (!sec.string_view) -> !sec.uint
    %2:3 = "sec.int.binary_checked"(%0, %1) <{kind = "add"}> : (!sec.uint, !sec.uint) -> (!sec.uint, i1, !sec.arithmetic_failure_reason)
    cf.cond_br %2#1, ^bb1(%2#2 : !sec.arithmetic_failure_reason), ^bb2
  ^bb1(%3: !sec.arithmetic_failure_reason):  // pred: ^bb0
    "sec.fail.arithmetic"(%3) {sec.operator = "+"} : (!sec.arithmetic_failure_reason) -> ()
  ^bb2:  // pred: ^bb0
    return %2#0 : !sec.uint
  }
  func.func @sec_fn_1(%arg0: !sec.ref<!sec.string> {sec.reference = {address_stability = "Stable", borrow_id = 25 : i32, lifetime_id = 25 : i32, provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}, %arg1: !sec.uint {sec.scalar_kind = "uint"}, %arg2: !sec.uint {sec.scalar_kind = "uint"}) -> !sec.string_view attributes {sec.extern = false, sec.function_id = "main::Param(ref string,uint,uint)", sec.parameter_names = ["text", "start", "end"], sec.return_origins = ["param#0[?]"], sec.source_name = "Param", sec.unsafe = false} {
    %0 = "sec.place.deref"(%arg0) : (!sec.ref<!sec.string>) -> !sec.place<!sec.string, "ro">
    %1 = "sec.string.rune_len"(%0) : (!sec.place<!sec.string, "ro">) -> !sec.uint
    %start, %length, %failed, %error = "sec.slice.range_check"(%1, %arg1, %arg2) <{end_exclusive = true, end_present = true, end_signed = false, start_present = true, start_signed = false}> : (!sec.uint, !sec.uint, !sec.uint) -> (!sec.uint, !sec.uint, i1, !sec.enum<identity = "core::RangeError", underlying = !sec.int, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>)
    cf.cond_br %failed, ^bb1, ^bb2
  ^bb1:  // pred: ^bb0
    "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
  ^bb2:  // pred: ^bb0
    %2 = "sec.string.view_borrow"(%0, %start, %length) <{address_stability = "Stable", borrow_id = 26 : i32, lifetime_id = 26 : i32, provenance = "parameter", source_borrow = 25 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow"}> {sec.slice_range_id = 1 : i32} : (!sec.place<!sec.string, "ro">, !sec.uint, !sec.uint) -> !sec.string_view
    return {sec.return_action = "non-owning"} %2 : !sec.string_view
  }
  func.func @sec_fn_2(%arg0: !sec.ref<!sec.string> {sec.reference = {address_stability = "Stable", borrow_id = 27 : i32, lifetime_id = 27 : i32, provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}) -> !sec.string_view attributes {sec.extern = false, sec.function_id = "main::Nested(ref string)", sec.parameter_names = ["text"], sec.return_origins = ["param#0[0..<1]"], sec.source_name = "Nested", sec.unsafe = false} {
    %0 = "sec.place.deref"(%arg0) : (!sec.ref<!sec.string>) -> !sec.place<!sec.string, "ro">
    %1 = "sec.string.rune_len"(%0) : (!sec.place<!sec.string, "ro">) -> !sec.uint
    %start, %length = "sec.slice.range_normalize"(%1) <{end_exclusive = false, end_present = false, end_signed = false, proof_kind = "source-open-bound", start_present = false, start_signed = false}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %2 = "sec.string.view_borrow"(%0, %start, %length) <{address_stability = "Stable", borrow_id = 28 : i32, lifetime_id = 28 : i32, provenance = "parameter", source_borrow = 27 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow"}> {sec.slice_range_id = 2 : i32} : (!sec.place<!sec.string, "ro">, !sec.uint, !sec.uint) -> !sec.string_view
    %3 = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    %4 = "sec.const.int"() <{value = "1"}> : () -> !sec.int
    %5 = "sec.slice.len"(%2) : (!sec.string_view) -> !sec.uint
    %start_0, %length_1, %failed, %error = "sec.slice.range_check"(%5, %3, %4) <{end_exclusive = true, end_present = true, end_signed = true, start_present = true, start_signed = true}> : (!sec.uint, !sec.int, !sec.int) -> (!sec.uint, !sec.uint, i1, !sec.enum<identity = "core::RangeError", underlying = !sec.int, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "InvalidRange", value = "0">, #sec.enum_case<ordinal = 1, name = "StartAfterEnd", value = "1">, #sec.enum_case<ordinal = 2, name = "OutOfBounds", value = "2">]>)
    cf.cond_br %failed, ^bb1, ^bb2
  ^bb1:  // pred: ^bb0
    "sec.fail.bounds"() {operation = "slice-range"} : () -> ()
  ^bb2:  // pred: ^bb0
    %6 = "sec.slice.reborrow_shared"(%2, %start_0, %length_1) <{address_stability = "Stable", borrow_id = 29 : i32, lifetime_id = 29 : i32, provenance = "parameter", source_borrow = 28 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow"}> {sec.slice_static_end_exclusive = "1", sec.slice_static_start = "0"} : (!sec.string_view, !sec.uint, !sec.uint) -> !sec.string_view
    return {sec.return_action = "non-owning"} %6 : !sec.string_view
  }
  func.func @sec_fn_3(%arg0: !sec.ref<!sec.string> {sec.reference = {address_stability = "Stable", borrow_id = 30 : i32, lifetime_id = 30 : i32, provenance = "parameter", validity_policy = "proven", validity_proof = "caller-owned-borrow"}}, %arg1: !sec.rune {sec.scalar_kind = "rune"}) -> (i1 {sec.scalar_kind = "bool"}) attributes {sec.extern = false, sec.function_id = "main::Contains(ref string,rune)", sec.parameter_names = ["input", "wanted"], sec.source_name = "Contains", sec.unsafe = false} {
    %0 = "sec.place.deref"(%arg0) : (!sec.ref<!sec.string>) -> !sec.place<!sec.string, "ro">
    %1 = "sec.string.rune_len"(%0) : (!sec.place<!sec.string, "ro">) -> !sec.uint
    %start, %length = "sec.slice.range_normalize"(%1) <{end_exclusive = false, end_present = false, end_signed = false, proof_kind = "source-open-bound", start_present = false, start_signed = false}> : (!sec.uint) -> (!sec.uint, !sec.uint)
    %2 = "sec.string.view_borrow"(%0, %start, %length) <{address_stability = "Stable", borrow_id = 31 : i32, lifetime_id = 31 : i32, provenance = "parameter", source_borrow = 30 : i32, validity_policy = "proven", validity_proof = "caller-owned-borrow"}> {sec.slice_range_id = 4 : i32} : (!sec.place<!sec.string, "ro">, !sec.uint, !sec.uint) -> !sec.string_view
    %3 = "sec.const.int"() <{value = "0"}> : () -> !sec.uint
    %4 = "sec.storage.declare"() {sec.mutable = true, sec.scalar_kind = "uint", sec.source_name = "ch.cursor", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32} : () -> !sec.storage<!sec.uint>
    "sec.storage.init"(%4, %3) : (!sec.storage<!sec.uint>, !sec.uint) -> ()
    cf.br ^bb1
  ^bb1:  // 2 preds: ^bb0, ^bb5
    %5 = "sec.storage.load"(%4) : (!sec.storage<!sec.uint>) -> !sec.uint
    %has, %value, %next = "sec.string_view.decode_next"(%2, %5) : (!sec.string_view, !sec.uint) -> (i1, !sec.rune, !sec.uint)
    cf.cond_br %has, ^bb2, ^bb3
  ^bb2:  // pred: ^bb1
    "sec.storage.store"(%4, %next) : (!sec.storage<!sec.uint>, !sec.uint) -> ()
    %6 = "sec.int.cmp"(%value, %arg1) <{predicate = "eq"}> : (!sec.rune, !sec.rune) -> i1
    cf.cond_br %6, ^bb4, ^bb5
  ^bb3:  // pred: ^bb1
    "sec.ref.end_borrow"(%2) <{borrow_id = 31 : i32, lifetime_id = 31 : i32}> : (!sec.string_view) -> ()
    %7 = "sec.const.bool"() <{value = false}> : () -> i1
    return %7 : i1
  ^bb4:  // pred: ^bb2
    %8 = "sec.const.bool"() <{value = true}> : () -> i1
    return %8 : i1
  ^bb5:  // pred: ^bb2
    cf.br ^bb1
  }
}

