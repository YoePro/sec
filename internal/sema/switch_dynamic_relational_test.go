package sema

import "testing"

// The unmatched path of a switch keeps the negation of every earlier pure
// alternative as a path fact, so a later case that dynamic operands already
// cover is proven unreachable (S3001) for subject and subjectless switches.
// A case entered by fallthrough from a matched clause, a still-possible case
// and impure alternatives stay reachable.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §§ 22–23 relational and dynamic overlap
//   - rules/tooling/diagnostics.md — § 21 "Proven unreachable and dead code"
func TestSwitchUnmatchedPathFactsCoverDynamicCases(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Probe() bool {
    return true
}

fn Repeat(x: int, limit: int) int {
    switch x {
    case < limit:
        return 1
    case < limit:
        return 2
    default:
        return 0
    }
}

fn Implied(x: int, limit: int) int {
    switch x {
    case <= limit:
        return 1
    case < limit:
        return 2
    case > limit:
        return 3
    default:
        return 0
    }
}

fn Subjectless(x: int, y: int) int {
    switch {
    case x < y:
        return 1
    case x < y:
        return 2
    default:
        return 0
    }
}

fn Fallthrough(x: int, limit: int) int {
    switch x {
    case < limit:
        fallthrough
    case < limit:
        return 2
    default:
        return 0
    }
}

fn Impure(x: int) int {
    switch {
    case Probe():
        return 1
    case Probe():
        return 2
    default:
        return 0
    }
}
`)
	assertSemaErrors(t, errors, []string{
		"unreachable statement at 13:9",
		"unreachable statement at 24:9",
		"unreachable statement at 37:9",
	})
}
