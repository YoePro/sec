package sema

import (
	"strings"
	"testing"
)

// MD-066 § 8 / MD-068 § 9(2) decimal core methods: total roundings and
// Normalize return the receiver type; Rescale and RoundTo report
// PrecisionError, Abs OverflowError and Clamp RangeError; arities and
// argument types are checked.
// Rules: rules/library/core-library.md §§ 10.2, 10.4.
func TestDecimalCoreMethods(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

enum RoundingMode {
    NearestEven
    TowardZero
}

enum OverflowError error {
    Overflow
}

fn Use(value: decimal, other: decimal, wide: decimal128) Result[decimal, PrecisionError] {
    let normal := value.Normalize()
    let whole := value.Floor() + value.Ceiling() + value.Truncate() + value.Round()
    let ordered := value.Min(other).Max(normal)
    let positive := try value.Abs() {
        Err(OverflowError.Overflow) => 0m
    }
    let bounded := try value.Clamp(0m, 10m) {
        Err(_) => 0m
    }
    let wideRounded := try wide.RoundTo(2, RoundingMode.TowardZero) {
        Err(_) => 0m
    }
    discard whole
    discard ordered
    discard positive
    discard bounded
    discard wideRounded
    let rescaled := try value.Rescale(4)
    discard rescaled
    return value.RoundTo(2)
}
`)
	assertSemaErrors(t, errs, nil)
	members := map[string]CompilerKnownMember{}
	for _, member := range CompilerKnownMembersForType(builtinType("decimal128"), false) {
		members[member.Name] = member
	}
	if members["RoundTo"].ID != "CKM-DECIMAL128-ROUNDTO" || members["Normalize"].Result.Name != "decimal128" {
		t.Fatalf("decimal128 members = %+v", members)
	}

	invalid := analyzeSourceRaw(t, `
module main

fn Bad(value: decimal) void {
    discard value.Rescale()
    discard value.RoundTo(2, 3)
    discard value.Clamp(1m)
}
`)
	for _, want := range []string{
		"decimal.Rescale expects 1 arguments, got 0",
		"decimal.RoundTo needs the core RoundingMode type",
		"decimal.Clamp expects 2 arguments, got 1",
	} {
		found := false
		for _, err := range invalid {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, invalid)
		}
	}
}
