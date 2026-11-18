// RUN: not sec-mlir-opt %s -split-input-file --sec-verify-storage-transitions -o /dev/null 2>&1 | FileCheck %s

// SEC-MLIR-P18 sections 30-31, 37, 58-67, 129-130: storage-domain lifecycle
// and growth invariants provable within one function.

// CHECK: names backing domain #2 but its owner uses #1
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @domains(%i: !sec.int) {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    "sec.storage.end_domain"(%p) <{domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    "sec.storage.end_domain"(%p) <{domain = 2 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    return
  }
}

// -----

// CHECK: depends on an owner after its storage.end_domain
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @after() -> !sec.uint {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    "sec.storage.end_domain"(%p) <{domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    %n = "sec.dynamic_array.len"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> !sec.uint
    return %n : !sec.uint
  }
}

// -----

// CHECK: must directly follow the end_domain or advance_epoch of the same owner and domain
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @reclaim() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    "sec.storage.reclaim"(%p) <{transition = "reclaim", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    return
  }
}

// -----

// CHECK: advances an epoch only after a true has_backing test of the same owner
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @advance() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    %spare = "sec.dynamic_array.has_capacity"(%p) <{count = "1"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    cf.cond_br %spare, ^done, ^grow
  ^grow:
    %fa, %ra = "sec.dynamic_array.grow_backing"(%p) <{count = "1", transition = "advance-epoch", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> (i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">]>)
    cf.cond_br %fa, ^done, ^advanced
  ^advanced:
    "sec.storage.advance_epoch"(%p) <{transition = "advance-epoch", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    cf.br ^done
  ^done:
    return
  }
}

// -----

// CHECK: uses an element Place or slice of an owner across its grow_backing
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @borrow(%i: !sec.int) -> !sec.int {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    %ok = "sec.dynamic_array.index_in_bounds"(%p, %i) <{index_signed = true}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> i1
    cf.cond_br %ok, ^element, ^done
  ^element:
    %e = "sec.dynamic_array.element_place"(%p, %i) <{index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded", backing_domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> !sec.place<!sec.int, "rw">
    %has = "sec.dynamic_array.has_backing"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    cf.cond_br %has, ^existing, ^done
  ^existing:
    %spare = "sec.dynamic_array.has_capacity"(%p) <{count = "1"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    cf.cond_br %spare, ^done, ^grow
  ^grow:
    %fb, %rb = "sec.dynamic_array.grow_backing"(%p) <{count = "1", transition = "advance-epoch", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> (i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">]>)
    cf.cond_br %fb, ^done, ^read
  ^read:
    %v = "sec.place.read"(%e) <{action = "copy-trivial"}> : (!sec.place<!sec.int, "rw">) -> !sec.int
    return %v : !sec.int
  ^done:
    return %i : !sec.int
  }
}

// -----

// CHECK: disagrees with the reclamation authority or plan of its domain
module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @facts() {
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    %has = "sec.dynamic_array.has_backing"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    cf.cond_br %has, ^existing, ^absent
  ^absent:
    %fc, %rc = "sec.dynamic_array.grow_backing"(%p) <{count = "1", transition = "establish-domain", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> (i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">]>)
    cf.cond_br %fc, ^done, ^established
  ^established:
    "sec.storage.establish_domain"(%p) <{transition = "establish-domain", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "Arena", stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    cf.br ^done
  ^existing:
    cf.br ^done
  ^done:
    return
  }
}

