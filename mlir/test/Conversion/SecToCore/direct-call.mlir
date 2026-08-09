// RUN: sec-mlir-opt %s --sec-lower-trivial-core --mlir-print-debuginfo | FileCheck %s

module {
  func.func @callee(%value: si32) -> si32 attributes {sec.extern = false} {
    return %value : si32
  }
  func.func @side_effect() attributes {sec.extern = false} {
    return
  }
  func.func @caller(%value: si32) -> si32 attributes {sec.extern = false} {
    "sec.call.direct"() <{callee = @side_effect}> {sec.argument_actions = []} : () -> ()
    %result = "sec.call.direct"(%value) <{callee = @callee}> {sec.argument_actions = ["copy-trivial"]} : (si32) -> si32 loc("call.sec":8:12)
    return %result : si32
  }
}

// CHECK-LABEL: func.func @caller
// CHECK: call @side_effect() : () -> ()
// CHECK: %[[RESULT:.*]] = call @callee(%arg0) : (si32) -> si32 loc(#[[CALL_LOC:loc[0-9]+]])
// CHECK: return %[[RESULT]] : si32
// CHECK-NOT: sec.call.direct
// CHECK-NOT: sec.argument_actions
// CHECK: #[[CALL_LOC]] = loc("call.sec":8:12)
