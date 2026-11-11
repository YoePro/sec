package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Compound updates of constrained fields and properties are checked like a
// constrained local: the try form uses the ContractError channel (handled or
// propagated); without try the update is valid, panics with ContractFailure
// on violation and reports A2011 (owner decision 2026-11-11); and a fallible
// setter on a constrained property cannot silently drop one of its two
// failure channels.
//
// Rules:
//   - rules/types/contracts.md — § 12(2)–(3)
//   - rules/declarations/properties.md — § 7 "Compound assignment"
//   - rules/errors/errorhandling.md — § 23 "Fallible assignment"
func TestConstrainedMemberCompoundAssignment(t *testing.T) {
	prelude := `module main

type Percent int range 0..100

type SpeedError enum error { TooFast }

type Gauge struct {
    _value: Percent,
}

impl Gauge {
    property Level: Percent {
        get {
            return self._value
        }
        set value {
            self._value = value
        }
    }
    property Checked: Percent {
        get {
            return self._value
        }
        try set value SpeedError {
            self._value = value
        }
    }
    fn Inside() int {
        try Level += 1 {
            Err(error) => return 1
        }
        try self._value -= 1 {
            Err(error) => return 2
        }
        return 0
    }
}

`
	valid := prelude + `fn Field(g: ref mut Gauge) int {
    try g._value += 10 {
        Err(error) => return 1
    }
    return 0
}

fn Property(g: ref mut Gauge) Result[void, ContractError] {
    try g.Level *= 2
    return Ok()
}

fn Converted(g: ref mut Gauge, n: int) int {
    try g._value = Percent(n) {
        Err(error) => return 1
    }
    return 0
}

fn Literal(g: ref mut Gauge) void {
    g._value = 50
    g.Level = 40
}
`
	if errors := analyzeSourceRaw(t, valid); len(errors) != 0 {
		t.Fatalf("valid constrained member updates: %v", errors)
	}

	// Untried compound updates are valid and each reports one A2011.
	untried := prelude + `fn F(g: ref mut Gauge) void {
    g._value += 10
    g.Level *= 2
}

impl extends Gauge {
    fn Bump() void {
        self._value += 1
        Level -= 1
    }
}
`
	analyzer, all := analyzeSourceWithAnalyzerRaw(t, untried)
	if len(all) != 0 {
		t.Fatalf("untried constrained updates rejected: %v", all)
	}
	advisories := []string{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.UncheckedContractMayPanic {
			advisories = append(advisories, warning.Message)
		}
	}
	for _, want := range []string{"compound update of g._value", "compound update of g.Level", "compound update of self._value", "compound update of Level"} {
		found := false
		for _, message := range advisories {
			found = found || strings.HasPrefix(message, want+" is checked against the contracts of Percent")
		}
		if !found {
			t.Errorf("no A2011 for %q in %v", want, advisories)
		}
	}

	cases := []struct {
		name string
		body string
		id   string
		want string
	}{
		{"fallible constrained property mixes channels", `fn F(g: ref mut Gauge) int {
    try g.Checked += 1 {
        Err(error) => return 1
    }
    return 0
}`, "", "would mix ContractError with SpeedError"},
		{"propagation needs ContractError channel", `fn F(g: ref mut Gauge) Result[void, SpeedError] {
    try g._value += 1
    return Ok()
}`, "", "propagates ContractError with return Err"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) && (test.id == "" || err.ID == test.id) {
					return
				}
			}
			t.Fatalf("no %s diagnostic containing %q; got %v", test.id, test.want, errors)
		})
	}
}

// Constrained array elements and referents follow the same two forms as
// locals and fields: try selects ContractError for replacement and compound
// update, and an untried compound update is valid, panics on violation and
// reports A2011 at the target.
//
// Rules:
//   - rules/types/contracts.md — § 12(2)–(3)
//   - rules/errors/errorhandling.md — § 23 "Fallible assignment"
func TestConstrainedElementAndReferentAssignment(t *testing.T) {
	source := `module main

type Percent int range 0..100

fn GetInt() int {
    return 42
}

fn Tried(r: ref mut Percent) Result[int, ContractError] {
    let mut levels: Percent[3] := [1, 2, 3]
    try levels[1] = Percent(GetInt())
    try levels[2] += 5
    try *r += 1
    try *r = Percent(GetInt()) {
        Err(error) => return Ok(1)
    }
    return Ok(0)
}

fn Untried(r: ref mut Percent) void {
    let mut levels: Percent[3] := [1, 2, 3]
    levels[0] += 1
    *r -= 1
    levels[1] = 4
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 0 {
		t.Fatalf("constrained element/referent updates rejected: %v", errors)
	}
	got := []string{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.UncheckedContractMayPanic {
			got = append(got, warning.Message)
		}
	}
	if len(got) != 2 || !strings.HasPrefix(got[0], "compound update of levels[0] is checked") || !strings.HasPrefix(got[1], "compound update of *r is checked") {
		t.Fatalf("A2011 advisories = %q", got)
	}

	// Without a contract the element is not a fallible target.
	plain := `module main

fn F() Result[void, ContractError] {
    let mut values: int[2] := [1, 2]
    try values[0] = 3
    return Ok()
}
`
	if errors := analyzeSourceRaw(t, plain); len(errors) != 1 || errors[0].ID != diagnostics.TryAssignmentWithoutFallibleTarget {
		t.Fatalf("plain element try = %v", errors)
	}
}
