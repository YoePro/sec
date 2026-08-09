// RUN: sec-mlir-opt %s -split-input-file -verify-diagnostics

module {
  func.func @signed_overflow() {
    // expected-error @+1 {{integer value is not representable by result type}}
    %value = "sec.const.int"() <{value = "170141183460469231731687303715884105728"}> : () -> si128
    return
  }
}

// -----

module {
  func.func @unsigned_negative() {
    // expected-error @+1 {{unsigned result cannot contain a negative value}}
    %value = "sec.const.int"() <{value = "-1"}> : () -> ui256
    return
  }
}
