// RUN: sec-mlir-opt %s --split-input-file --sec-lower-integer-enums --verify-diagnostics | FileCheck %s

// An explicit enum-to-integer conversion is exact when the target is the
// carrier or every declared case value fits the target; otherwise it is
// named as not lowered (enums.md §§ 7, 13; MD-104).

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  // CHECK-LABEL: func.func @sec_fn_0
  // CHECK: "sec.numeric.convert_exact"(%{{.*}}) : (si8) -> si64
  // CHECK-NOT: sec.enum.to_integer
  func.func @sec_fn_0(%l: !sec.enum<identity = "main::Level", underlying = si8, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "Cold", value = "-10">, #sec.enum_case<ordinal = 1, name = "Hot", value = "90">]>) -> si64 attributes {sec.function_id = "main::Widen(main::Level)", sec.source_name = "Widen", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["l"]} {
    %v = "sec.enum.to_integer"(%l) : (!sec.enum<identity = "main::Level", underlying = si8, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "Cold", value = "-10">, #sec.enum_case<ordinal = 1, name = "Hot", value = "90">]>) -> si64
    return %v : si64
  }
  // CHECK-LABEL: func.func @sec_fn_1
  // CHECK-NOT: sec.enum.to_integer
  // CHECK: return %{{.*}} : si8
  func.func @sec_fn_1(%l: !sec.enum<identity = "main::Level", underlying = si8, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "Cold", value = "-10">, #sec.enum_case<ordinal = 1, name = "Hot", value = "90">]>) -> si8 attributes {sec.function_id = "main::Carrier(main::Level)", sec.source_name = "Carrier", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["l"]} {
    %v = "sec.enum.to_integer"(%l) : (!sec.enum<identity = "main::Level", underlying = si8, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "Cold", value = "-10">, #sec.enum_case<ordinal = 1, name = "Hot", value = "90">]>) -> si8
    return %v : si8
  }
}

// -----

module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["main.sec"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @sec_fn_0(%l: !sec.enum<identity = "main::Level", underlying = si8, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "Cold", value = "-10">, #sec.enum_case<ordinal = 1, name = "Hot", value = "90">]>) -> ui8 attributes {sec.function_id = "main::Unsigned(main::Level)", sec.source_name = "Unsigned", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["l"]} {
    // expected-error @+1 {{does not lower 'sec.enum.to_integer' to 'ui8' (a case value does not fit) yet}}
    %v = "sec.enum.to_integer"(%l) : (!sec.enum<identity = "main::Level", underlying = si8, representation = "integer", bitWidth = 0, cases = [#sec.enum_case<ordinal = 0, name = "Cold", value = "-10">, #sec.enum_case<ordinal = 1, name = "Hot", value = "90">]>) -> ui8
    return %v : ui8
  }
}
