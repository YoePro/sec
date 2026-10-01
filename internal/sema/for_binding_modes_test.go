package sema

import (
	"os"
	"strings"
	"testing"
)

// ref and ref mut loop bindings type as references with the iterable's
// origin over sequential collections, slices, sets, and maps.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §6 "Shared element iteration", §7 "Mutable element iteration"
//   - rules/control-flow/flowcontrol_for.md — §14 "Sequential collections", §17 "Slices", §20 "Sets", §21 "Maps"
func TestForBindingModesAccepted(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/for_binding_modes_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(input)); len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
}

// Binding modes the iterable category or source authority does not provide
// are rejected without undefined-name cascades, and reference bindings keep
// the iterable's origin for escape analysis.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §5 "Plain by-value iteration", §14 "Sequential collections", §17 "Slices"
//   - rules/control-flow/flowcontrol_for.md — §19 "Strings", §20 "Sets", §21 "Maps", §40 "Unsupported forms"
//   - rules/memory/borrowing.md — §7 "Mutability authority"
func TestForBindingModeDiagnostics(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/for_binding_modes_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	wants := []struct {
		line    int
		message string
	}{
		{19, "plain loop binding handle would copy each Handle element"},
		{21, "ref loop bindings are not provided for synthesized range values"},
		{23, "string iteration yields decoded rune values"},
		{25, "the sequential index binding is an int, not an element reference"},
		{27, "ref mut loop binding requires mutable element authority, but the iterable is a shared reference"},
		{29, "set elements cannot be mutably borrowed during iteration"},
		{31, "map keys cannot be mutably borrowed during iteration"},
		{37, "ref mut loop binding requires a mutable iterable source"},
		{40, "ref loop binding requires a stable addressable iterable"},
		{47, "cannot return reference to local variable values"},
		{49, "cannot return reference to local variable values"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d", errors, len(wants))
	}
	for i, want := range wants {
		if errors[i].Line != want.line || !strings.Contains(errors[i].Message, want.message) {
			t.Fatalf("error %d = %+v, want %q at line %d", i, errors[i], want.message, want.line)
		}
	}
}

// Float and decimal ranges require an explicit step, and an unsigned range
// cannot use a negative step.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §27 "Unsigned descending ranges", §28 "Float and decimal ranges"
func TestForRangeStepRequirements(t *testing.T) {
	input := `
module main

fn Ranges(limit: uint) void {
    for value in 0.001..<0.002 {
    }
    for value in 0.0g..<1.0g {
    }
    for value in limit..0u step -1 {
    }
    for value in 0.001..<0.002 step 0.0001 {
    }
    for value in 0.0g..<1.0g step 0.5g {
    }
}
`
	errors := analyzeSourceRaw(t, input)
	wants := []struct {
		line    int
		message string
	}{
		{5, "decimal range iteration requires an explicit step"},
		{7, "float range iteration requires an explicit step"},
		{9, "unsigned range uint cannot use negative step -1"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d", errors, len(wants))
	}
	for i, want := range wants {
		if errors[i].Line != want.line || !strings.Contains(errors[i].Message, want.message) {
			t.Fatalf("error %d = %+v, want %q at line %d", i, errors[i], want.message, want.line)
		}
	}
}
