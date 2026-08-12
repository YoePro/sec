// RUN: sec-mlir-opt --sec-verify-match-cfg %s -o /dev/null

module {
  func.func @match_cfg(%condition: i1) -> i32 {
    cf.cond_br %condition, ^body, ^residual {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "enum-value"}
  ^body:
    %value = arith.constant 1 : i32
    cf.br ^merge(%value : i32) {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "body-exit", sec.match_pattern_kind = "enum-value"}
  ^residual:
    "sec.unreachable"() <{reason = "exhaustive-match-fallthrough"}> {sec.synthesized = true, sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "residual", sec.match_pattern_kind = "enum-value"} : () -> ()
  ^merge(%result: i32):
    return %result : i32
  }
}
