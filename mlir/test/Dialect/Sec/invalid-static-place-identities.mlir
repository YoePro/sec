// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-116 §§4(6),9(3): identities cannot contradict declarations across functions.
module {
"sec.static.global"() <{initial = "10", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true, sec.source_name = "main::Count"} : () -> ()
"sec.static.global"() <{initial = "0", is_mutable = true, sym_name = "sec_static_2", type = si32}> {sec.static_id = 5 : i32, sec.address_taken = true, sec.source_name = "main::Total"} : () -> ()
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_3", type = si32}> {sec.static_id = 6 : i32, sec.source_name = "main::Plain"} : () -> ()
  func.func @first() {
    %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
  func.func @storage() {
    // expected-error@+1 {{conflicting storage identity or address space}}
    %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 14 : i32, storage_identity = 9 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
"sec.static.global"() <{initial = "10", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true, sec.source_name = "main::Count"} : () -> ()
"sec.static.global"() <{initial = "0", is_mutable = true, sym_name = "sec_static_2", type = si32}> {sec.static_id = 5 : i32, sec.address_taken = true, sec.source_name = "main::Total"} : () -> ()
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_3", type = si32}> {sec.static_id = 6 : i32, sec.source_name = "main::Plain"} : () -> ()
  func.func @first() {
    %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
  func.func @space() {
    // expected-error@+1 {{conflicting storage identity or address space}}
    %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 14 : i32, storage_identity = 7 : i32, address_space = "mmio"}> : () -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
"sec.static.global"() <{initial = "10", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true, sec.source_name = "main::Count"} : () -> ()
"sec.static.global"() <{initial = "0", is_mutable = true, sym_name = "sec_static_2", type = si32}> {sec.static_id = 5 : i32, sec.address_taken = true, sec.source_name = "main::Total"} : () -> ()
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_3", type = si32}> {sec.static_id = 6 : i32, sec.source_name = "main::Plain"} : () -> ()
  func.func @first() {
    %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
  func.func @alias() {
    // expected-error@+1 {{distinct static Place roots share storage identity}}
    %root = "sec.place.static"() <{global = @sec_static_2, place_root_id = 14 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
}

// -----

module {
"sec.static.global"() <{initial = "10", is_mutable = false, sym_name = "sec_static_1", type = si32}> {sec.static_id = 4 : i32, sec.address_taken = true, sec.source_name = "main::Count"} : () -> ()
"sec.static.global"() <{initial = "0", is_mutable = true, sym_name = "sec_static_2", type = si32}> {sec.static_id = 5 : i32, sec.address_taken = true, sec.source_name = "main::Total"} : () -> ()
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "sec_static_3", type = si32}> {sec.static_id = 6 : i32, sec.source_name = "main::Plain"} : () -> ()
  func.func @first() {
    %root = "sec.place.static"() <{global = @sec_static_1, place_root_id = 12 : i32, storage_identity = 7 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
  func.func @root() {
    // expected-error@+1 {{distinct static Place roots share root identity}}
    %root = "sec.place.static"() <{global = @sec_static_2, place_root_id = 12 : i32, storage_identity = 8 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
}
