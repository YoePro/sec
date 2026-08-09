// RUN: sec-mlir-opt %s | sec-mlir-opt | FileCheck %s

module {
  func.func private @foreign(si32) -> si32 attributes {sec.extern = true}

  func.func @callee(%value: si32) -> si32 attributes {sec.extern = false} {
    return %value : si32
  }

  func.func @caller(%value: si32, %condition: i1) -> si32 attributes {sec.extern = false} {
    cf.cond_br %condition, ^direct, ^foreign
  ^direct:
    %direct_result = "sec.call.direct"(%value) <{callee = @callee}> {sec.argument_actions = ["copy-trivial"]} : (si32) -> si32
    cf.br ^exit(%direct_result : si32)
  ^foreign:
    %foreign_result = "sec.call.foreign"(%value) <{callee = @foreign}> {sec.argument_actions = ["copy-trivial"]} : (si32) -> si32
    cf.br ^exit(%foreign_result : si32)
  ^exit(%result: si32):
    return %result : si32
  }
}

// CHECK: cf.cond_br
// CHECK: "sec.call.direct"
// CHECK: "sec.call.foreign"
// CHECK: cf.br
