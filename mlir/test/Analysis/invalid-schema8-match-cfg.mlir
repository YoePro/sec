// RUN: not sec-mlir-opt --sec-verify-match-cfg %s -o /dev/null 2>&1 | FileCheck %s

module {
  func.func @incomplete(%condition: i1) {
    cf.cond_br %condition, ^done, ^done {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern"}
  ^done:
    return
  }
}

// CHECK: incomplete Sec match provenance
