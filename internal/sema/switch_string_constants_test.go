package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestComputedStringSwitchCoverage normalizes only proven constant values,
// including decoded escapes and immutable bindings. Runtime calls and distinct
// Unicode scalar sequences remain distinct from compile-time duplicate proofs.
// Rules: rules/control-flow/flowcontrol_switch.md — §§20,25;
// rules/compiler/compile_time_evaluation.md — shared constant evaluation.
func TestComputedStringSwitchCoverage(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/switch_string_constants/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(source)); len(errors) != 0 {
		t.Fatal(errors)
	}
	source, err = os.ReadFile("../../testdata/sema/switch_string_constants/duplicate_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 3 {
		t.Fatalf("want three duplicate constants: %v", errors)
	}
	for i, lines := range [][2]int{{6, 5}, {11, 11}, {17, 16}} {
		d := errors[i]
		if d.ID != diagnostics.DuplicateSwitchCase || d.Line != lines[0] || d.PreviousLine != lines[1] || d.PreviousColumn == 0 || d.Help == "" || !strings.Contains(d.Message, "duplicate switch case value") {
			t.Fatalf("incorrect duplicate provenance: %+v", d)
		}
	}
}
