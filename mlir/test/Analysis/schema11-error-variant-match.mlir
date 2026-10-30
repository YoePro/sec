// RUN: sec-mlir-opt --sec-verify-match-cfg --split-input-file --verify-diagnostics %s
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.4 (MD-008, MD-029).

!E = !sec.union<identity = "main::FileError", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Denied", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Encoding", kind = "single", payload = !sec.int, fields = []>]>
!R = !sec.result<!sec.int, !E>

// A result-err-variant arm tests the variant of the error unwrapped from the
// one Result subject and projects its payload on the variant's true edge.
func.func @error_variant(%subject: !R) -> !sec.int {
  %is_err = "sec.result.is_err"(%subject) {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"} : (!R) -> i1
  cf.cond_br %is_err, ^test, ^next {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"}
^test:
  %error = "sec.result.unwrap_err"(%subject) {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"} : (!R) -> !E
  %variant = "sec.union.is_variant"(%error) <{variant = 1 : i32}> {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"} : (!E) -> i1
  cf.cond_br %variant, ^body, ^next {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"}
^body:
  %payload = "sec.union.unwrap_payload"(%error) <{payloadAction = "copy-trivial", variant = 1 : i32}> {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"} : (!E) -> !sec.int
  "func.return"(%payload) {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "body-exit", sec.match_pattern_kind = "result-err-variant"} : (!sec.int) -> ()
^next:
  "sec.unreachable"() <{reason = "exhaustive-match-fallthrough"}> {sec.synthesized = true, sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "residual", sec.match_pattern_kind = "result-err-variant"} : () -> ()
}

// -----

!E = !sec.union<identity = "main::FileError", typeArguments = [], variants = [#sec.union_variant<index = 0, name = "Denied", kind = "empty", fields = []>, #sec.union_variant<index = 1, name = "Encoding", kind = "single", payload = !sec.int, fields = []>]>
!R = !sec.result<!sec.int, !E>

// The variant test must use the error of the match subject, not another value.
func.func @foreign_error(%subject: !R, %other: !E) -> !sec.int {
  %is_err = "sec.result.is_err"(%subject) {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"} : (!R) -> i1
  cf.cond_br %is_err, ^test, ^next {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"}
^test:
  // expected-error @+1 {{error variant test must use the error unwrapped from the match subject}}
  %variant = "sec.union.is_variant"(%other) <{variant = 1 : i32}> {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"} : (!E) -> i1
  cf.cond_br %variant, ^body, ^next {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "pattern", sec.match_pattern_kind = "result-err-variant"}
^body:
  %value = "sec.const.int"() <{value = "1"}> : () -> !sec.int
  "func.return"(%value) {sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "body-exit", sec.match_pattern_kind = "result-err-variant"} : (!sec.int) -> ()
^next:
  "sec.unreachable"() <{reason = "exhaustive-match-fallthrough"}> {sec.synthesized = true, sec.match_id = 1 : i32, sec.match_arm_index = 0 : i32, sec.match_stage = "residual", sec.match_pattern_kind = "result-err-variant"} : () -> ()
}
