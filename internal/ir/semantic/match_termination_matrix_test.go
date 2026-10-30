package semantic

import "testing"

// TestMatchTerminationMatrixIR keeps the frontend's normalized exhaustive
// plans through IR construction, including nested matches and unsafe blocks.
// Rules: rules/control-flow/flowcontrol_match.md — §23;
// rules/compiler/semantic_ir.md — §71.
func TestMatchTerminationMatrixIR(t *testing.T) {
	module := buildForFixture(t, "../../../testdata/control_flow/match_termination/valid.sec")
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"Union": 2, "Optional": 2, "Aliases": 2, "Bits": 2, "Nested": 3, "NestedUnsafe": 2, "PanicArm": 1, "Guarded": 3} {
		function := loopFunction(t, module, name)
		if got := countOperations(function, OpReturn); got != want {
			t.Fatalf("%s returns=%d want=%d", name, got, want)
		}
		if name == "PanicArm" && countOperations(function, OpPanic) != 1 {
			t.Fatal("lost non-returning panic arm")
		}
	}
}
