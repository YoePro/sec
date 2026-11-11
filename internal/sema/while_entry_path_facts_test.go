package sema

import "testing"

// A while condition that dominating path facts or first-entry constants decide
// at entry routes through the canonical unreachable-code policy: proven false,
// the body is unreachable (S3001) and the loop runs zero times; proven true
// with a body that never reaches the next test or a break, the code after the
// loop is unreachable and satisfies return analysis. Bodies that fall
// through, continue or break, and undecided conditions keep the ordinary exit.
//
// Rules:
//   - rules/control-flow/flowcontrol_while.md — §§ 19–20, 28 condition proof and continuation
//   - rules/tooling/diagnostics.md — § 21 "Proven unreachable and dead code"
func TestWhileEntryDecidedByPathFacts(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn EntryFalse(x: int) int {
    if x > 10 {
        while x < 5 {
            return 1
        }
    }
    return 0
}

fn EntryTrue(x: int) int {
    if x > 10 {
        while x > 5 {
            return 1
        }
        return 2
    }
    return 0
}

fn EntryTrueSatisfiesReturn(x: int) int {
    if x > 10 {
        while x > 5 {
            return 1
        }
    }
    return 0
}

fn ConstantEntry() int {
    let mut i := 0
    while i < 3 {
        return i
    }
    return 9
}

fn FallsThrough(seed: int) int {
    let mut x := seed
    if x > 10 {
        while x > 5 {
            x -= 1
        }
        return x
    }
    return 0
}

fn Breaks(x: int, stop: bool) int {
    if x > 10 {
        while x > 5 {
            if stop {
                break
            }
            return 1
        }
        return 2
    }
    return 0
}

fn Undecided(x: int) int {
    while x > 5 {
        return 1
    }
    return 0
}
`)
	assertSemaErrors(t, errors, []string{
		"unreachable statement at 7:13",
		"unreachable statement at 18:9",
		"unreachable statement at 37:5",
	})
}
