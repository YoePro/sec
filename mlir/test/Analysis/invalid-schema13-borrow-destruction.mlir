// RUN: sec-mlir-opt %s -split-input-file --sec-verify-ownership -verify-diagnostics
// RUN: sec-mlir-opt %s -split-input-file --sec-verify-places -verify-diagnostics
// Rules: rules/memory/destruction.md §18(3); P17 §117. Explicit P15 ends, not inferred last use.
!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @shared_live(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @mutable_live(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_mut"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref_mut<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @one_path_live(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    cf.cond_br %flag, ^ended, ^live
  ^ended:
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    cf.br ^join
  ^live:
    cf.br ^join
  ^join:
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @forwarded_holder(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    cf.br ^next(%r : !sec.ref<!sec.int>)
  ^next(%alias: !sec.ref<!sec.int>):
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%alias) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @wrong_end_identity(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    "sec.ref.end_borrow"(%r) <{borrow_id = 9 : i32, lifetime_id = 9 : i32}> : (!sec.ref<!sec.int>) -> ()
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @copy_outlives_parent(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    // expected-note@+1 {{payload borrow holder begins here}}
    %copy = "sec.ref.copy_shared"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref<!sec.int>) -> !sec.ref<!sec.int>
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%copy) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @move_outlives_source(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    %r = "sec.ref.borrow_mut"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref_mut<!sec.int>
    // expected-note@+1 {{payload borrow holder begins here}}
    %copy = "sec.ref.move"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.ref_mut<!sec.int>) -> !sec.ref_mut<!sec.int>
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%copy) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref_mut<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @child_outlives_parent(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    // expected-note@+1 {{payload borrow holder begins here}}
    %child = "sec.ref.reborrow_shared"(%r) <{borrow_id = 2 : i32, lifetime_id = 2 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local", source_borrow = 1 : i32}> : (!sec.ref<!sec.int>) -> !sec.ref<!sec.int>
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%child) <{borrow_id = 2 : i32, lifetime_id = 2 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @loop_live(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    cf.br ^loop
  ^loop:
    cf.cond_br %flag, ^loop, ^done
  ^done:
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @live_value(%subject: !u, %other: !u, %flag: i1) {
    %p = "sec.place.value"(%subject) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!u) -> !sec.place<!u, "ro">
    %some = "sec.union.is_variant"(%subject) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "ro">) -> !sec.place<!sec.int, "ro">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_value"(%subject) <{cause = "temporary-end", destruction_plan = "u"}> : (!u) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @live_discard(%subject: !u, %other: !u, %flag: i1) {
    %p = "sec.place.value"(%subject) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!u) -> !sec.place<!u, "ro">
    %some = "sec.union.is_variant"(%subject) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "ro">) -> !sec.place<!sec.int, "ro">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "ro">) -> !sec.ref<!sec.int>
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.discard"(%subject) <{discard_origin = "explicit", destruction_plan = "u"}> : (!u) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @live_replace(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+2 {{payload borrow holder begins here}}
    "sec.cleanup.track_owned"(%p) <{cleanup_action_id = 1 : i32, cleanup_scope_id = 1 : i32, cleanup_registration_ordinal = 1 : i32, destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.replace_place"(%p, %other) <{destruction_plan = "u", old_cleanup_action_id = 1 : i32, new_cleanup_action_id = 1 : i32}> : (!sec.place<!u, "rw">, !u) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @storage_alias(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    %alias_place = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%alias_place) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
!pair = !sec.struct<identity = "main::Pair", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "left", type = !u, tags = []>, #sec.struct_field<ordinal = 1, name = "right", type = !u, tags = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "p", type = !pair, kind = "trivial"}, {id = "u", type = !u, kind = "trivial"}]} {
  func.func @parent_destroy(%input: !pair) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "pair", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!pair>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!pair>) -> !sec.place<!pair, "rw">
    "sec.own.initialize_place"(%p, %input) : (!sec.place<!pair, "rw">, !pair) -> ()
    %left = "sec.place.field"(%p) <{field = 0 : i32}> : (!sec.place<!pair, "rw">) -> !sec.place<!u, "rw">
    %right = "sec.place.field"(%p) <{field = 1 : i32}> : (!sec.place<!pair, "rw">) -> !sec.place<!u, "rw">
    %value = "sec.place.read"(%right) <{action = "copy-trivial"}> : (!sec.place<!u, "rw">) -> !u
    %some = "sec.union.is_variant"(%value) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%right) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "p"}> : (!sec.place<!pair, "rw">) -> ()
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @wrong_borrow_only(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    "sec.ref.end_borrow"(%r) <{borrow_id = 9 : i32, lifetime_id = 1 : i32}> : (!sec.ref<!sec.int>) -> ()
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    return
  ^none:
    return
  }
}

// -----

!u = !sec.union<identity = "main::Maybe", typeArguments = [!sec.int], variants = [#sec.union_variant<index = 0, name = "Some", kind = "single", payload = !sec.int, fields = []>, #sec.union_variant<index = 1, name = "None", kind = "empty", fields = []>]>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "borrow_destroy", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "u", type = !u, kind = "trivial"}]} {
  func.func @wrong_lifetime_only(%subject: !u, %other: !u, %flag: i1) {
    %storage = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "subject", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!u>
    %p = "sec.place.storage"(%storage) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!u>) -> !sec.place<!u, "rw">
    "sec.own.initialize_place"(%p, %subject) : (!sec.place<!u, "rw">, !u) -> ()
    %observed = "sec.storage.load"(%storage) : (!sec.storage<!u>) -> !u
    %some = "sec.union.is_variant"(%observed) <{variant = 0 : i32}> : (!u) -> i1
    cf.cond_br %some, ^some, ^none
  ^some:
    %payload = "sec.place.union_payload"(%p) <{variant = 0 : i32}> : (!sec.place<!u, "rw">) -> !sec.place<!sec.int, "rw">
    // expected-note@+1 {{payload borrow holder begins here}}
    %r = "sec.ref.borrow_shared"(%payload) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> : (!sec.place<!sec.int, "rw">) -> !sec.ref<!sec.int>
    "sec.ref.end_borrow"(%r) <{borrow_id = 1 : i32, lifetime_id = 9 : i32}> : (!sec.ref<!sec.int>) -> ()
    // expected-error@+1 {{cannot destroy storage while a payload borrow is live}}
    "sec.own.destroy_place"(%p) <{cause = "temporary-end", destruction_plan = "u"}> : (!sec.place<!u, "rw">) -> ()
    return
  ^none:
    return
  }
}
