package sema

import (
	"strings"
	"testing"
)

// Switch duplicate and overlap reasoning compares case values in the
// subject's own representation: quantity literals with exactly the subject's
// unit and conversions into the subject's named semantic type are compile-time
// values; incompatible units stay rejected and compatible-but-different units
// are not compared by raw coefficient.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §§ 20–21 duplicates and overlaps, § 26 named semantic types and units
func TestSwitchOverlapIsUnitAndNamedTypeAware(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

unit m int physical
unit s int physical

impl <m> {
    Dimension: [length^1]
}
impl <s> {
    Dimension: [time^1]
}

type Speed int

fn Distance(distance: int<m>) int {
    switch distance {
    case 0<m>..<100<m>:
        return 1
    case 50<m>..<60<m>:
        return 2
    case 55<m>:
        return 3
    case 1<s>:
        return 4
    case 100<m>:
        return 5
    default:
        return 0
    }
}

fn Named(speed: Speed) int {
    switch speed {
    case Speed(1):
        return 1
    case Speed(1):
        return 2
    case Speed(0)..<Speed(5):
        return 3
    case Speed(7):
        return 4
    default:
        return 0
    }
}
`)
	want := []string{
		"unreachable switch case; previous case already covers this condition at 20:15",
		"switch case value 55<m> is already covered by previous case at 22:10",
		"switch case must be compatible with subject type int<m>, got int<s> at 24:10",
		"duplicate switch case value Speed(1) at 37:10",
		"switch case range overlaps previous case at 39:18",
	}
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	if len(errors) != len(want) {
		t.Fatalf("want %d diagnostics, got:\n%s", len(want), joined)
	}
	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Fatalf("missing %q in:\n%s", w, joined)
		}
	}
}
