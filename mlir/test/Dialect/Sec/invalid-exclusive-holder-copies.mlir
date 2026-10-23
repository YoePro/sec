// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// P15 section 7; P16 sections 39,114; copy_move section 4.1.
module {
  func.func @copy(%p: !sec.place<!sec.named<"device::Alias", !sec.ref_mut<si32>>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.named<"device::Alias", !sec.ref_mut<si32>>, "ro">) -> !sec.named<"device::Alias", !sec.ref_mut<si32>>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.distinct<"device::Unique", !sec.ref_mut<si32>>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.distinct<"device::Unique", !sec.ref_mut<si32>>, "ro">) -> !sec.distinct<"device::Unique", !sec.ref_mut<si32>>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.array<!sec.ref_mut<si32>, "2">, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.array<!sec.ref_mut<si32>, "2">, "ro">) -> !sec.array<!sec.ref_mut<si32>, "2">
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>, "ro">) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.ref_mut<si32>, fields = []>]>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.ref_mut<si32>, fields = []>]>, "ro">) -> !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.ref_mut<si32>, fields = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.result<!sec.ref_mut<si32>, !sec.core_error<"device::Error">>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.result<!sec.ref_mut<si32>, !sec.core_error<"device::Error">>, "ro">) -> !sec.result<!sec.ref_mut<si32>, !sec.core_error<"device::Error">>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.storage<!sec.ref_mut<si32>>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.ref_mut<si32>>) -> !sec.ref_mut<si32>
    return
  }
}


// -----

module {
  func.func private @target(!sec.ref_mut<si32>) attributes {sec.extern = false}
  func.func @copy(%v: !sec.ref_mut<si32>) {
    // expected-error@+1 {{argument action must be mutable-reference}}
    "sec.call.direct"(%v) <{callee = @target}> {sec.argument_actions = ["copy-trivial"]} : (!sec.ref_mut<si32>) -> ()
    return
  }
}


// -----

module {
  func.func @copy(%a: !sec.array<!sec.ref_mut<si32>, "2">, %i: si32) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.array.extract"(%a, %i) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<!sec.ref_mut<si32>, "2">, si32) -> !sec.ref_mut<si32>
    return
  }
}


// -----

module {
  func.func @copy(%a: !sec.array<!sec.ref_mut<si32>, "2">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.array.construct"(%a) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["2"]}> : (!sec.array<!sec.ref_mut<si32>, "2">) -> !sec.array<!sec.ref_mut<si32>, "2">
    return
  }
}


// -----

module {
  func.func @copy(%a: !sec.array<!sec.ref_mut<si32>, "2">, %i: si32, %v: !sec.ref_mut<si32>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.array.replace"(%a, %i, %v) <{bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<!sec.ref_mut<si32>, "2">, si32, !sec.ref_mut<si32>) -> !sec.array<!sec.ref_mut<si32>, "2">
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.ref_mut<si32>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.struct.construct"(%v) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!sec.ref_mut<si32>) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.struct.extract"(%v) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>) -> !sec.ref_mut<si32>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.struct.spread_fields"(%v) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>) -> !sec.ref_mut<si32>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>, %field: !sec.ref_mut<si32>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.struct.replace_field"(%v, %field) <{field = 0 : i32}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>, !sec.ref_mut<si32>) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.ref_mut<si32>, tags = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.ref_mut<si32>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.union.construct"(%v) <{fieldNames = [], payloadActions = ["copy-trivial"], variant = 0 : i32}> : (!sec.ref_mut<si32>) -> !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.ref_mut<si32>, fields = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.ref_mut<si32>, fields = []>]>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.union.unwrap_payload"(%v) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.ref_mut<si32>, fields = []>]>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.place<!sec.slice_mut<si32>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.slice_mut<si32>, "ro">) -> !sec.slice_mut<si32>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.named<"device::Alias", !sec.slice_mut<si32>>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.named<"device::Alias", !sec.slice_mut<si32>>, "ro">) -> !sec.named<"device::Alias", !sec.slice_mut<si32>>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.distinct<"device::Unique", !sec.slice_mut<si32>>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.distinct<"device::Unique", !sec.slice_mut<si32>>, "ro">) -> !sec.distinct<"device::Unique", !sec.slice_mut<si32>>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.array<!sec.slice_mut<si32>, "2">, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.array<!sec.slice_mut<si32>, "2">, "ro">) -> !sec.array<!sec.slice_mut<si32>, "2">
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>, "ro">) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.slice_mut<si32>, fields = []>]>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.slice_mut<si32>, fields = []>]>, "ro">) -> !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.slice_mut<si32>, fields = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.place<!sec.result<!sec.slice_mut<si32>, !sec.core_error<"device::Error">>, "ro">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.result<!sec.slice_mut<si32>, !sec.core_error<"device::Error">>, "ro">) -> !sec.result<!sec.slice_mut<si32>, !sec.core_error<"device::Error">>
    return
  }
}


// -----

module {
  func.func @copy(%p: !sec.storage<!sec.slice_mut<si32>>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.slice_mut<si32>>) -> !sec.slice_mut<si32>
    return
  }
}


// -----

module {
  func.func private @target(!sec.slice_mut<si32>) attributes {sec.extern = false}
  func.func @copy(%v: !sec.slice_mut<si32>) {
    // expected-error@+1 {{argument action must be mutable-reference}}
    "sec.call.direct"(%v) <{callee = @target}> {sec.argument_actions = ["copy-trivial"]} : (!sec.slice_mut<si32>) -> ()
    return
  }
}


// -----

module {
  func.func @copy(%a: !sec.array<!sec.slice_mut<si32>, "2">, %i: si32) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.array.extract"(%a, %i) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<!sec.slice_mut<si32>, "2">, si32) -> !sec.slice_mut<si32>
    return
  }
}


// -----

module {
  func.func @copy(%a: !sec.array<!sec.slice_mut<si32>, "2">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.array.construct"(%a) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["2"]}> : (!sec.array<!sec.slice_mut<si32>, "2">) -> !sec.array<!sec.slice_mut<si32>, "2">
    return
  }
}


// -----

module {
  func.func @copy(%a: !sec.array<!sec.slice_mut<si32>, "2">, %i: si32, %v: !sec.slice_mut<si32>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.array.replace"(%a, %i, %v) <{bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<!sec.slice_mut<si32>, "2">, si32, !sec.slice_mut<si32>) -> !sec.array<!sec.slice_mut<si32>, "2">
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.slice_mut<si32>) {
    // expected-error@+1 {{mutable slice aggregate construction requires move-aware lowering}}
    %copy = "sec.struct.construct"(%v) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!sec.slice_mut<si32>) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>) {
    // expected-error@+1 {{cannot copy a mutable slice through struct extract}}
    %copy = "sec.struct.extract"(%v) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>) -> !sec.slice_mut<si32>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>) {
    // expected-error@+1 {{cannot copy a mutable slice through struct spread}}
    %copy = "sec.struct.spread_fields"(%v) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>) -> !sec.slice_mut<si32>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>, %field: !sec.slice_mut<si32>) {
    // expected-error@+1 {{cannot copy a mutable slice through struct replacement}}
    %copy = "sec.struct.replace_field"(%v, %field) <{field = 0 : i32}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>, !sec.slice_mut<si32>) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.slice_mut<si32>, tags = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.slice_mut<si32>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.union.construct"(%v) <{fieldNames = [], payloadActions = ["copy-trivial"], variant = 0 : i32}> : (!sec.slice_mut<si32>) -> !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.slice_mut<si32>, fields = []>]>
    return
  }
}


// -----

module {
  func.func @copy(%v: !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.slice_mut<si32>, fields = []>]>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.union.unwrap_payload"(%v) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.slice_mut<si32>, fields = []>]>) -> !sec.slice_mut<si32>
    return
  }
}

// -----

module {
  func.func @empty(%p: !sec.storage<!sec.array<!sec.ref_mut<si32>, "0">>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.array<!sec.ref_mut<si32>, "0">>) -> !sec.array<!sec.ref_mut<si32>, "0">
    return
  }
}

// -----

module {
  func.func @empty(%p: !sec.storage<!sec.array<!sec.slice_mut<si32>, "0">>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.array<!sec.slice_mut<si32>, "0">>) -> !sec.array<!sec.slice_mut<si32>, "0">
    return
  }
}

// -----

module {
  func.func @empty(%p: !sec.storage<!sec.array<!sec.hw_instance<"device::Control">, "0">>) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.array<!sec.hw_instance<"device::Control">, "0">>) -> !sec.array<!sec.hw_instance<"device::Control">, "0">
    return
  }
}

// -----

module {
  func.func private @target(!sec.array<!sec.ref_mut<si32>, "2">) attributes {sec.extern = false}
  func.func @argument(%a: !sec.array<!sec.ref_mut<si32>, "2">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    "sec.call.direct"(%a) <{callee = @target}> {sec.argument_actions = ["copy-trivial"]} : (!sec.array<!sec.ref_mut<si32>, "2">) -> ()
    return
  }
}

// -----

module {
  func.func @field(%a: !sec.union<identity = "main::Holder", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "fields", fields = [#sec.union_field<name = "holder", type = !sec.ref_mut<si32>>]>]>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.union.unwrap_field"(%a) <{variant = 0 : i32, field = "holder", payloadAction = "copy-trivial"}> : (!sec.union<identity = "main::Holder", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "fields", fields = [#sec.union_field<name = "holder", type = !sec.ref_mut<si32>>]>]>) -> !sec.ref_mut<si32>
    return
  }
}

// -----

module {
  func.func private @target(!sec.array<!sec.slice_mut<si32>, "2">) attributes {sec.extern = false}
  func.func @argument(%a: !sec.array<!sec.slice_mut<si32>, "2">) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    "sec.call.direct"(%a) <{callee = @target}> {sec.argument_actions = ["copy-trivial"]} : (!sec.array<!sec.slice_mut<si32>, "2">) -> ()
    return
  }
}

// -----

module {
  func.func @field(%a: !sec.union<identity = "main::Holder", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "fields", fields = [#sec.union_field<name = "holder", type = !sec.slice_mut<si32>>]>]>) {
    // expected-error@+1 {{cannot trivially copy an exclusive reference holder}}
    %copy = "sec.union.unwrap_field"(%a) <{variant = 0 : i32, field = "holder", payloadAction = "copy-trivial"}> : (!sec.union<identity = "main::Holder", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Some", kind = "fields", fields = [#sec.union_field<name = "holder", type = !sec.slice_mut<si32>>]>]>) -> !sec.slice_mut<si32>
    return
  }
}
