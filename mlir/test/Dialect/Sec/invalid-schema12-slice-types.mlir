// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics -o /dev/null

module {
  // expected-error@+1 {{sec.slice element must be a non-void semantic value}}
  func.func private @none(!sec.slice<none>)
}

// -----

module {
  // expected-error@+1 {{sec.slice_mut element must be a non-void semantic value}}
  func.func private @never(!sec.slice_mut<!sec.never>)
}

// -----

module {
  // expected-error@+1 {{sec.slice element must be a non-void semantic value}}
  func.func private @storage(!sec.slice<!sec.storage<i32>>)
}

// -----

module {
  // expected-error@+1 {{sec.slice_mut element must be a non-void semantic value}}
  func.func private @place(!sec.slice_mut<!sec.place<i32, "rw">>)
}

// -----

module {
  // expected-error@+1 {{sec.slice element must be a non-void semantic value}}
  func.func private @nominal_place(!sec.slice<!sec.named<"NotAValue", !sec.place<i32, "ro">>>)
}

// -----

module {
  // expected-error@+1 {{sec.slice element must be a non-void semantic value}}
  func.func private @signature(!sec.slice<(i32) -> i32>)
}

// -----

module {
  // expected-error@+1 {{expected '>'}}
  func.func private @length(!sec.slice<i32, 4>)
}

// -----

module {
  // expected-error@+1 {{expected '>'}}
  func.func private @epoch(!sec.slice_mut<i32, "epoch">)
}
