// RUN: sec-mlir-opt --mlir-print-debuginfo %s | FileCheck %s

module {
  func.func @located(%value: !sec.named<"main::Speed", i64>)
      -> !sec.named<"main::Speed", i64> {
    return %value : !sec.named<"main::Speed", i64> loc("sample.sec":12:8)
  }
}

// CHECK: return %{{.*}} : !sec.named<"main::Speed", i64> loc(#[[RETURN_LOC:loc[0-9]+]])
// CHECK: #[[RETURN_LOC]] = loc("sample.sec":12:8)
