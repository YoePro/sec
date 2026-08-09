// RUN: sec-mlir-opt %s --sec-resolve-scalar-layout | FileCheck %s

module attributes {dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 32>>} {
  func.func private @foreign(!sec.int) -> !sec.uint attributes {sec.extern = true}
  func.func @callee(%value: !sec.int) -> !sec.int attributes {sec.extern = false} {
    return %value : !sec.int
  }
  func.func @resolve(%i: !sec.int, %u: !sec.uint, %f: !sec.float, %c: !sec.char, %r: !sec.rune) -> !sec.int attributes {sec.extern = false} {
    %one = "sec.const.int"() <{value = "1"}> : () -> !sec.int
    %float = "sec.const.float"() <{lexeme = "1.25"}> : () -> !sec.float
    %direct = "sec.call.direct"(%i) <{callee = @callee}> {sec.argument_actions = ["copy-trivial"]} : (!sec.int) -> !sec.int
    %foreign = "sec.call.foreign"(%i) <{callee = @foreign}> {sec.argument_actions = ["copy-trivial"]} : (!sec.int) -> !sec.uint
    cf.br ^exit(%direct : !sec.int)
  ^exit(%result: !sec.int):
    return %result : !sec.int
  }
  func.func @named(%value: !sec.named<"main::Count", !sec.int>) -> !sec.named<"main::Count", !sec.int> {
    return %value : !sec.named<"main::Count", !sec.int>
  }
}

// CHECK: func.func private @foreign(si32) -> ui32
// CHECK: func.func @callee(%arg0: si32) -> si32
// CHECK: func.func @resolve(%arg0: si32, %arg1: ui32, %arg2: f64, %arg3: ui8, %arg4: ui32) -> si32
// CHECK: "sec.const.int"() <{value = "1"}> : () -> si32
// CHECK: "sec.const.float"() <{lexeme = "1.25"}> : () -> f64
// CHECK: "sec.call.direct"(%arg0) {{.*}} : (si32) -> si32
// CHECK: "sec.call.foreign"(%arg0) {{.*}} : (si32) -> ui32
// CHECK: cf.br ^bb1(%{{.*}} : si32)
// CHECK: ^bb1(%{{.*}}: si32):
// CHECK: !sec.named<"main::Count", si32>
// CHECK-NOT: !sec.int
// CHECK-NOT: !sec.uint
// CHECK-NOT: unrealized_conversion_cast
