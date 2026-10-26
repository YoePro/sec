// RUN: sec-mlir-opt %s --split-input-file --sec-lower-floats --verify-diagnostics
// P20 §§75–80: standalone lowering without scalar layout must diagnose the
// missing predecessor, never assert while casting an unresolved !sec.float.
func.func @remainder(%x: !sec.float) -> !sec.float {
  // expected-error@+1 {{requires resolved binary32/binary64 scalar layout before float lowering}}
  %v = "sec.float.binary"(%x, %x) {kind = "rem"} : (!sec.float, !sec.float) -> !sec.float
  return %v : !sec.float
}
// -----
func.func @classify(%x: !sec.float) -> i1 {
  // expected-error@+1 {{requires resolved binary32/binary64 scalar layout before float lowering}}
  %v = "sec.float.classify"(%x) {kind = "normal"} : (!sec.float) -> i1
  return %v : i1
}
// -----
func.func @round(%x: !sec.float) -> !sec.float {
  // expected-error@+1 {{requires resolved binary32/binary64 scalar layout before float lowering}}
  %v = "sec.float.round"(%x) {kind = "nearest-even"} : (!sec.float) -> !sec.float
  return %v : !sec.float
}
