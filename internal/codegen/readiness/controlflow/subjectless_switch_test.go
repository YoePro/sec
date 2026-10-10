package controlflow_test

import "testing"

// TestSubjectlessSwitchExecution verifies reached-only, exactly-once boolean
// alternatives; default/unmatched continuation; independent scopes; empty and
// nested switches; and break/continue targeting the enclosing loop.
// Rules: rules/control-flow/flowcontrol_switch.md §§5–6, 12–15, 18–19, 31, 39;
// rules/corrections/applied/correction24-20260823.md — "Normative requirement".
func TestSubjectlessSwitchExecution(t *testing.T) {
	executeControlFlowFixture(t, "subjectless_switch", "17 subjectless switch scenarios passed\n")
}
