package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// A compound assignment whose known operands overflow follows its underlying
// operator: the non-blocking S1023 warning with checked runtime semantics,
// not a representability error. Known values come only from facts that hold
// on every path: an assignment in a branch, arm or loop body invalidates the
// outer constant at the join, and a loop body does not see the pre-loop value
// of a binding it assigns (the `while remaining > 0 { remaining -= 1 }` of
// sec/core int parsing was rejected as "value -1 overflows uint").
//
// Rules: rules/foundations/operators.md — "Compile-time overflow",
// "Compound arithmetic failure"; rules/types/types.md — int and uint.
func TestCompoundAssignmentConstantOverflow(t *testing.T) {
	source := `module main

fn Definite() uint {
    let mut x: uint := 0
    x -= 1
    return x
}

fn Wide() int {
    let mut a: int := 2000000000
    a += 2000000000
    return a
}

fn Countdown(exponent: uint) uint {
    let mut remaining: uint := 0
    while remaining > 0 {
        remaining -= 1
    }
    return remaining
}

fn AfterBranch(flag: bool) uint {
    let mut exponent: uint := 0
    if flag {
        exponent = 5
    }
    exponent -= 1
    return exponent
}

fn AfterLoop(n: uint) uint {
    let mut exponent: uint := 0
    for i in 0u..<n {
        exponent = exponent * 10 + 1
    }
    let mut remaining := exponent
    while remaining > 0 {
        remaining -= 1
    }
    return remaining
}

fn AfterArm(maybe: Option[uint]) uint {
    let mut value: uint := 0
    match maybe {
        Some(found) => {
            value = found
        }
        None => {}
    }
    value -= 1
    return value
}
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatal(parsed.Errors())
	}
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 32})
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("compound overflow must not be an error: %v", errors)
	}
	lines := map[int]bool{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OperatorIntegerOverflow {
			if !strings.Contains(warning.Message, "may overflow") {
				t.Errorf("unexpected S1023 message %q", warning.Message)
			}
			lines[warning.Line] = true
		}
	}
	// Definite (x -= 1) and Wide (a += 2000000000 on a 32-bit int) warn;
	// no path-dependent value does.
	if len(lines) != 2 || !lines[5] || !lines[11] {
		t.Errorf("S1023 lines = %v, want exactly 5 and 11; warnings %v", lines, analyzer.Warnings())
	}
}
