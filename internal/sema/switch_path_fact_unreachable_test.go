package sema

import "testing"

// A switch clause whose every alternative is proven false by dominating path
// facts is reported with the canonical S3001 unreachable-statement diagnostic,
// as for if conditions: value, range and relational alternatives of a subject
// switch and subjectless boolean alternatives. A clause entered by
// fallthrough, a still-possible alternative and a descending range that
// normalizes to a possible one stay reachable.
//
// Rules:
//   - rules/tooling/diagnostics.md — § 21(1), (3)–(6) "Proven unreachable and dead code"
//   - rules/control-flow/flowcontrol_switch.md — § 23 "Dynamic overlap"
func TestSwitchClausesDecidedByPathFactsAreUnreachable(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Value(x: int) int {
    if x == 1 {
        switch x {
        case 2:
            return 2
        case 1:
            return 1
        default:
            return 0
        }
    }
    return 3
}

fn Range(x: int) int {
    if x > 10 {
        switch x {
        case 0..<5:
            return 2
        case 30..15:
            return 4
        case < 3, 50:
            return 5
        default:
            return 0
        }
    }
    return 3
}

fn Subjectless(x: int) int {
    if x < 0 {
        switch {
        case x > 5:
            return 1
        default:
            return 0
        }
    }
    return 3
}

fn Fallthrough(x: int) int {
    if x == 1 {
        switch x {
        case 1:
            fallthrough
        case 2:
            return 2
        default:
            return 0
        }
    }
    return 3
}
`)
	assertSemaErrors(t, errors, []string{
		"unreachable statement at 8:13",
		"unreachable statement at 22:13",
		"unreachable statement at 38:13",
	})
}
