// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module attributes {
  dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>,
  sec.dialect_version = 9 : i32,
  sec.semantic_ir_version = 1 : i32,
  sec.module_id = "invalid-schema9-structs",
  sec.source_files = [],
  sec.target_os = "linux",
  sec.target_arch = "amd64",
  sec.target_triple = "x86_64-pc-linux-gnu",
  sec.target_abi = "gnu",
  sec.target_profile = "hosted",
  sec.target_endianness = "little"
} {
  func.func @wrong_construct(%value: si32) {
    // expected-error@+1 {{field operands, origins, and actions must match the stored-field count}}
    %bad = "sec.struct.construct"(%value) <{field_actions = [], field_origins = ["explicit"]}> : (si32) -> !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>
    return
  }
}

// -----

module {
  func.func @bad_action(%value: si32) {
    // expected-error@+1 {{field actions contains an invalid value}}
    %bad = "sec.struct.construct"(%value) <{field_actions = ["move"], field_origins = ["explicit"]}> : (si32) -> !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>
    return
  }
}

// -----

module {
  func.func @wrong_origin_count(%value: si32) {
    // expected-error@+1 {{field operands, origins, and actions must match the stored-field count}}
    %bad = "sec.struct.construct"(%value) <{field_actions = ["construct-direct"], field_origins = []}> : (si32) -> !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>
    return
  }
}

// -----

module {
  func.func @wrong_operand_type(%value: ui32) {
    // expected-error@+1 {{field operands must use declaration order and exact types}}
    %bad = "sec.struct.construct"(%value) <{field_actions = ["construct-direct"], field_origins = ["explicit"]}> : (ui32) -> !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>
    return
  }
}

// -----

module {
  func.func @bad_spread(%source: !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) {
    // expected-error@+1 {{results must use declaration-order field types}}
    %bad = "sec.struct.spread_fields"(%source) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) -> ui32
    return
  }
}

// -----

module {
  func.func @bad_spread_count(%source: !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) {
    // expected-error@+1 {{results and actions must match the stored-field count}}
    "sec.struct.spread_fields"(%source) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) -> ()
    return
  }
}

// -----

module {
  func.func @bad_extract(%source: !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) {
    // expected-error@+1 {{struct field ordinal is out of range}}
    %bad = "sec.struct.extract"(%source) <{action = "copy-trivial", field = 1 : i32}> : (!sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) -> si32
    return
  }
}

// -----

module {
  func.func @bad_extract_action(%source: !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) {
    // expected-error@+1 {{schema 9 struct extract action must be copy-trivial}}
    %bad = "sec.struct.extract"(%source) <{action = "move", field = 0 : i32}> : (!sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) -> si32
    return
  }
}

// -----

module {
  func.func @bad_extract_type(%source: !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) {
    // expected-error@+1 {{result type must exactly match the selected field type}}
    %bad = "sec.struct.extract"(%source) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>) -> ui32
    return
  }
}

// -----

module {
  func.func @bad_replace(%source: !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>, %replacement: ui32) {
    // expected-error@+1 {{replacement type must exactly match the selected field type}}
    %bad = "sec.struct.replace_field"(%source, %replacement) <{field = 0 : i32}> : (!sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>, ui32) -> !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>
    return
  }
}

// -----

module {
  func.func @bad_replace_result(%source: !sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>, %replacement: si32) {
    // expected-error@+1 {{source and result struct types must match exactly}}
    %bad = "sec.struct.replace_field"(%source, %replacement) <{field = 0 : i32}> : (!sec.struct<identity = "main::One", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>, si32) -> !sec.struct<identity = "main::Other", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = []>]>
    return
  }
}

// -----

module {
  func.func @bad_replace_source(%source: si32, %replacement: si32) {
    // expected-error@+1 {{value must have !sec.struct type}}
    %bad = "sec.struct.replace_field"(%source, %replacement) <{field = 0 : i32}> : (si32, si32) -> si32
    return
  }
}

// -----

module {
  // expected-error@+1 {{sec.struct field ordinals must be contiguous from zero}}
  func.func @bad_ordinal(%source: !sec.struct<identity = "main::Bad", typeArguments = [], fields = [#sec.struct_field<ordinal = 1, name = "value", type = si32, tags = []>]>)
}

// -----

module {
  // expected-error@+2 {{failed to parse Sec_StructType parameter 'fields'}}
  // expected-error@+1 {{struct field tags must be #sec.struct_tag attributes}}
  func.func @bad_tag(%source: !sec.struct<identity = "main::Bad", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "value", type = si32, tags = [#sec.enum_case<ordinal = 0, name = "zero", value = "0">]>]>)
}
