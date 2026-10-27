// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

// MD-116 § 9(3): malformed and forged static Place roots and static
// identities are rejected by the operation and attribute verifiers.

func.func @unknown_symbol() {
  // expected-error@+1 {{global must name a sec.static.global}}
  %p = "sec.place.static"() <{global = @missing, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  return
}

// -----

func.func @not_a_static() {
  // expected-error@+1 {{global must name a sec.static.global}}
  %p = "sec.place.static"() <{global = @not_a_static, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  return
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.address_taken = true} : () -> ()
func.func @missing_id() {
  // expected-error@+1 {{the referenced static requires sec.static_id}}
  %p = "sec.place.static"() <{global = @s, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  return
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i32} : () -> ()
func.func @missing_address_taken() {
  // expected-error@+1 {{the referenced static requires sec.address_taken = true}}
  %p = "sec.place.static"() <{global = @s, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  return
}

// -----

// expected-error@+1 {{sec.address_taken must be the boolean marker true}}
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i32, sec.address_taken = false} : () -> ()

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i32, sec.address_taken = true} : () -> ()
func.func @wrong_type() {
  // expected-error@+1 {{Place element type must match the static's type}}
  %p = "sec.place.static"() <{global = @s, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si64, "ro">
  return
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i32, sec.address_taken = true} : () -> ()
func.func @rw_immutable() {
  // expected-error@+1 {{a rw Place root requires a mutable static}}
  %p = "sec.place.static"() <{global = @s, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "rw">
  return
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i32, sec.address_taken = true} : () -> ()
func.func @zero_root() {
  // expected-error@+1 {{place_root_id must be non-zero}}
  %p = "sec.place.static"() <{global = @s, place_root_id = 0 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
  return
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i32, sec.address_taken = true} : () -> ()
func.func @target_space() {
  // expected-error@+1 {{address_space must be ordinary or mmio}}
  %p = "sec.place.static"() <{global = @s, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "target:sram"}> : () -> !sec.place<si32, "ro">
  return
}

// -----

// expected-error@+1 {{sec.static_id must be a nonzero i32 on sec.static.global}}
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 0 : i32} : () -> ()

// -----

// expected-error@+1 {{sec.static_id must be a nonzero i32 on sec.static.global}}
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i64} : () -> ()

// -----

func.func @id_elsewhere() attributes {sec.static_id = 1 : i32} {
  // expected-error@-1 {{sec.static_id must be a nonzero i32 on sec.static.global}}
  return
}

// -----

"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "a", type = si32}> {sec.static_id = 2 : i32} : () -> ()
// expected-error@+1 {{duplicate sec.static_id 2}}
"sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "b", type = si32}> {sec.static_id = 2 : i32} : () -> ()

// -----

// Schemas 1-10 do not admit static Place roots (MD-116 § 1(4)).
// expected-error@+1 {{Places, direct references and their operations require schema 11}}
module attributes {sec.dialect_version = 10 : i32} {
  "sec.static.global"() <{initial = "1", is_mutable = false, sym_name = "s", type = si32}> {sec.static_id = 1 : i32, sec.address_taken = true} : () -> ()
  func.func @old_schema() {
    %p = "sec.place.static"() <{global = @s, place_root_id = 1 : i32, storage_identity = 1 : i32, address_space = "ordinary"}> : () -> !sec.place<si32, "ro">
    return
  }
}
