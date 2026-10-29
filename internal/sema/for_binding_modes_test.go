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

// A ref or ref mut loop binding holds a borrow of the iterable until its last
// required use in each iteration, so conflicting access to the iterated
// collection is rejected while the binding is live and accepted after it.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §6 "Shared element iteration", §7 "Mutable element iteration"
//   - rules/memory/borrowing.md — §§ 8.1(11), 14.5(2), 18(4)
func TestForBindingBorrowsIterable(t *testing.T) {
	input := `
module main

type Counter struct {
    count: int,
}

fn Live(counters: ref mut Counter[]) void {
    let mut values: Counter[3] := [Counter{count: 0}, Counter{count: 1}, Counter{count: 2}]
    for ref mut value in values {
        let other: int := values[0].count
        value.count = other
    }
    for ref value in values {
        values[1].count = 3
        let current: int := value.count
    }
    for ref mut counter in counters {
        counters[0].count = 1
        counter.count = 2
    }
}

fn Ended(counters: ref mut Counter[]) void {
    let mut values: Counter[3] := [Counter{count: 0}, Counter{count: 1}, Counter{count: 2}]
    for ref mut value in values {
        value.count = 1
        let other: int := values[0].count
    }
    for index, ref mut value in values {
        if index == 1 {
            break
        }
        value.count = index
    }
    values[0].count = 5
    for ref counter in counters {
        for ref inner in counters {
            let total: int := counter.count + inner.count
        }
    }
    for value in values {
        values[0].count = value.count
    }
}
`
	errors := analyzeSourceRaw(t, input)
	wants := []struct {
		line    int
		message string
	}{
		{11, "cannot read values[0].count while it is mutably borrowed"},
		{15, "cannot assign to values[1].count while it is shared borrowed"},
		{19, "cannot assign to counters[0].count while it is mutably borrowed"},
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

// The sequential index stays an ordinary int, map and set iteration accept
// exactly the permitted reference modes, and discard bindings place no copy
// requirement on the discarded component.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §9 "Discard bindings", §14 "Sequential collections", §20 "Sets", §21 "Maps"
func TestForBindingPositionModes(t *testing.T) {
	input := `
module main

@noCopy
type Handle struct {
    id: int,
}

fn Use(index: int) void {
}

fn Positions(handles: ref Handle[], shared: ref map[string, int], m: ref mut map[string, Handle], s: ref set[int]) void {
    for _ in handles {
    }
    for index, _ in handles {
        Use(index)
    }
    for index, ref handle in handles {
        let copy: int := index
    }
    for ref key, ref mut value in shared {
    }
    for ref key, ref mut value in m {
    }
    for key, ref value in m {
    }
    for key, value in m {
    }
    for _, _ in m {
    }
    for value in s {
    }
    for ref value in s {
    }
    for ref mut index, ref handle in handles {
    }
}
`
	errors := analyzeSourceRaw(t, input)
	wants := []struct {
		line    int
		message string
	}{
		{21, "ref mut loop binding requires mutable element authority, but the iterable is a shared reference"},
		{27, "plain loop binding value would copy each Handle element"},
		{35, "the sequential index binding is an int, not an element reference"},
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
