// RUN: sec-mlir-opt %s --sec-verify-dynamic-arrays --sec-verify-places --sec-verify-destruction-plans -o /dev/null

// SEC-MLIR-P18 sections 81-96: schema-14 owning dynamic arrays, the growth
// protocol, guarded element Places, storage transitions and the
// dynamic-array destruction plan (end-domain, then reclaim per plan).

module attributes {sec.dialect_version = 14 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "arrays", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>, sec.destruction_plans = [{id = "destroy::int:int", type = !sec.int, kind = "trivial"}, {id = "destroy:array:dynamic::int:int", type = !sec.dynamic_array<!sec.int>, kind = "dynamic-array", element = "destroy::int:int", backing = ["end-domain", "reclaim-per-allocation-plan"]}]} {
  func.func @fill(%x: !sec.int) -> !sec.uint {
    %e = "sec.dynamic_array.empty"() : () -> !sec.dynamic_array<!sec.int>
    %s = "sec.storage.declare"() {sec.mutable = true, sec.source_name = "values", sec.storage_class = "local-automatic", sec.storage_id = 1 : i32, sec.address_taken = true} : () -> !sec.storage<!sec.dynamic_array<!sec.int>>
    %p = "sec.place.storage"(%s) <{place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : (!sec.storage<!sec.dynamic_array<!sec.int>>) -> !sec.place<!sec.dynamic_array<!sec.int>, "rw">
    "sec.own.initialize_place"(%p, %e) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.dynamic_array<!sec.int>) -> ()
    %has = "sec.dynamic_array.has_backing"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    cf.cond_br %has, ^existing, ^absent
  ^absent:
    %f1, %r1 = "sec.dynamic_array.grow_backing"(%p) <{count = "1", transition = "establish-domain", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> (i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %f1, ^failed, ^established
  ^established:
    "sec.storage.establish_domain"(%p) <{transition = "establish-domain", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    cf.br ^append
  ^existing:
    %spare = "sec.dynamic_array.has_capacity"(%p) <{count = "1"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> i1
    cf.cond_br %spare, ^append, ^grow
  ^grow:
    %f2, %r2 = "sec.dynamic_array.grow_backing"(%p) <{count = "1", transition = "advance-epoch", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> (i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %f2, ^failed, ^advanced
  ^advanced:
    "sec.storage.advance_epoch"(%p) <{transition = "advance-epoch", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    "sec.storage.reclaim"(%p) <{transition = "reclaim", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    cf.br ^append
  ^append:
    "sec.dynamic_array.append_prepared"(%p, %x) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> ()
    %zero = "sec.const.int"() <{value = "0"}> : () -> !sec.int
    %ok = "sec.dynamic_array.index_in_bounds"(%p, %zero) <{index_signed = true}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> i1
    cf.cond_br %ok, ^element, ^failed
  ^element:
    %place = "sec.dynamic_array.element_place"(%p, %zero) <{index_signed = true, bounds_kind = "runtime-check", bounds_proof = "guarded", backing_domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">, !sec.int) -> !sec.place<!sec.int, "rw">
    %n = "sec.dynamic_array.len"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> !sec.uint
    %i = "sec.dynamic_array.len_int"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> !sec.int
    %c = "sec.dynamic_array.capacity_internal"(%p) : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> !sec.uint
    "sec.storage.end_domain"(%p) <{domain = 1 : i32}> : (!sec.place<!sec.dynamic_array<!sec.int>, "rw">) -> ()
    return %n : !sec.uint
  ^failed:
    %zero2 = "sec.const.int"() <{value = "0"}> : () -> !sec.uint
    return %zero2 : !sec.uint
  }
  func.func @allocate() {
    %a, %failed, %reason = "sec.dynamic_array.allocate"() <{count = "4", transition = "establish-domain", domain = 1 : i32, allocation = 1 : i32, allocation_context = 1 : i32, origin = "Allocator", relation = "Owned", authority = "OwningValue", reclamation = 1 : i32, stability = "Movable", space = "ordinary"}> : () -> (!sec.dynamic_array<!sec.int>, i1, !sec.enum<identity = "core::AllocationError", underlying = !sec.uint, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "OutOfMemory", value = "0">, #sec.enum_case<ordinal = 1, name = "Unsupported", value = "1">, #sec.enum_case<ordinal = 2, name = "InvalidSize", value = "2">, #sec.enum_case<ordinal = 3, name = "InvalidAlignment", value = "3">]>)
    cf.cond_br %failed, ^failure, ^success
  ^failure:
    return
  ^success:
    return
  }
}
