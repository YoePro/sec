// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null
// MD-077 section 3(5): copy-only operations cannot duplicate instance authority.

// -----

module {
  func.func @copy(%p: !sec.place<!sec.hw_instance<"device::Control">, "ro">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.hw_instance<"device::Control">, "ro">) -> !sec.hw_instance<"device::Control">
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.place<!sec.named<"device::Alias", !sec.hw_instance<"device::Control">>, "ro">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.named<"device::Alias", !sec.hw_instance<"device::Control">>, "ro">) -> !sec.named<"device::Alias", !sec.hw_instance<"device::Control">>
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.place<!sec.distinct<"device::Unique", !sec.hw_instance<"device::Control">>, "ro">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.distinct<"device::Unique", !sec.hw_instance<"device::Control">>, "ro">) -> !sec.distinct<"device::Unique", !sec.hw_instance<"device::Control">>
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.place<!sec.array<!sec.hw_instance<"device::Control">, "2">, "ro">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.array<!sec.hw_instance<"device::Control">, "2">, "ro">) -> !sec.array<!sec.hw_instance<"device::Control">, "2">
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.place<!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>, "ro">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>, "ro">) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.place<!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.hw_instance<"device::Control">, fields = []>]>, "ro">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.hw_instance<"device::Control">, fields = []>]>, "ro">) -> !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.hw_instance<"device::Control">, fields = []>]>
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.place<!sec.result<!sec.hw_instance<"device::Control">, !sec.core_error<"device::Error">>, "ro">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.place.read"(%p) <{action = "copy-trivial"}> : (!sec.place<!sec.result<!sec.hw_instance<"device::Control">, !sec.core_error<"device::Error">>, "ro">) -> !sec.result<!sec.hw_instance<"device::Control">, !sec.core_error<"device::Error">>
    return
  }
}

// -----

module {
  func.func @copy(%p: !sec.storage<!sec.hw_instance<"device::Control">>) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.storage.load"(%p) : (!sec.storage<!sec.hw_instance<"device::Control">>) -> !sec.hw_instance<"device::Control">
    return
  }
}

// -----

module {
  func.func private @target(!sec.hw_instance<"device::Control">) attributes {sec.extern = false}
  func.func @copy(%v: !sec.hw_instance<"device::Control">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    "sec.call.direct"(%v) <{callee = @target}> {sec.argument_actions = ["copy-trivial"]} : (!sec.hw_instance<"device::Control">) -> ()
    return
  }
}

// -----

module {
  func.func @copy(%a: !sec.array<!sec.hw_instance<"device::Control">, "2">, %i: si32) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.array.extract"(%a, %i) <{action = "copy-trivial", bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<!sec.hw_instance<"device::Control">, "2">, si32) -> !sec.hw_instance<"device::Control">
    return
  }
}

// -----

module {
  func.func @copy(%a: !sec.array<!sec.hw_instance<"device::Control">, "2">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.array.construct"(%a) <{segment_actions = ["copy-trivial"], segment_kinds = ["spread"], segment_lengths = ["2"]}> : (!sec.array<!sec.hw_instance<"device::Control">, "2">) -> !sec.array<!sec.hw_instance<"device::Control">, "2">
    return
  }
}

// -----

module {
  func.func @copy(%a: !sec.array<!sec.hw_instance<"device::Control">, "2">, %i: si32, %v: !sec.hw_instance<"device::Control">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.array.replace"(%a, %i, %v) <{bounds_kind = "proven-safe", bounds_proof = "range"}> : (!sec.array<!sec.hw_instance<"device::Control">, "2">, si32, !sec.hw_instance<"device::Control">) -> !sec.array<!sec.hw_instance<"device::Control">, "2">
    return
  }
}

// -----

module {
  func.func @copy(%v: !sec.hw_instance<"device::Control">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.struct.construct"(%v) <{field_actions = ["copy-trivial"], field_origins = ["explicit"]}> : (!sec.hw_instance<"device::Control">) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>
    return
  }
}

// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.struct.extract"(%v) <{action = "copy-trivial", field = 0 : i32}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>) -> !sec.hw_instance<"device::Control">
    return
  }
}

// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.struct.spread_fields"(%v) <{actions = ["copy-trivial"]}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>) -> !sec.hw_instance<"device::Control">
    return
  }
}

// -----

module {
  func.func @copy(%v: !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>, %field: !sec.hw_instance<"device::Control">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.struct.replace_field"(%v, %field) <{field = 0 : i32}> : (!sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>, !sec.hw_instance<"device::Control">) -> !sec.struct<identity = "device::Holder", typeArguments = [], fields = [#sec.struct_field<ordinal = 0, name = "Instance", type = !sec.hw_instance<"device::Control">, tags = []>]>
    return
  }
}

// -----

module {
  func.func @copy(%v: !sec.hw_instance<"device::Control">) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.union.construct"(%v) <{fieldNames = [], payloadActions = ["copy-trivial"], variant = 0 : i32}> : (!sec.hw_instance<"device::Control">) -> !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.hw_instance<"device::Control">, fields = []>]>
    return
  }
}

// -----

module {
  func.func @copy(%v: !sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.hw_instance<"device::Control">, fields = []>]>) {
    // expected-error@+1 {{cannot trivially copy hardware-instance authority}}
    %copy = "sec.union.unwrap_payload"(%v) <{payloadAction = "copy-trivial", variant = 0 : i32}> : (!sec.union<identity = "device::Choice", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Device", kind = "single", payload = !sec.hw_instance<"device::Control">, fields = []>]>) -> !sec.hw_instance<"device::Control">
    return
  }
}
