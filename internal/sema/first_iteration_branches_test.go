package sema

import (
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// The first-iteration constant domain joins across branches like ordinary
// constants: writes inside if/else, switch or match branches keep a value
// every continuing branch agrees on (an if without else also keeps the
// pre-branch path), while a one-sided change, a write in an inner loop, and
// an inner loop that may have run end the binding's first-iteration value, so
// no stale certain-overflow warning remains.
//
// Rules:
//   - rules/foundations/operators.md — "Compile-time overflow"
//   - rules/control-flow/flowcontrol_if.md — joins
func TestFirstIterationDomainJoinsAcrossBranches(t *testing.T) {
	const source = `module main

fn BothBranchesKeep(c: bool, d: bool) uint {
    let mut n: uint := 3
    while c {
        if d {
            n -= 0
        } else {
            n += 0
        }
        n -= 4
    }
    return n
}

fn OneSided(c: bool, d: bool) uint {
    let mut n: uint := 3
    while c {
        if d {
            n = 10
        }
        n -= 4
    }
    return n
}

fn InnerLoopWrite(c: bool, d: bool) uint {
    let mut n: uint := 3
    while c {
        if d {
            while d {
                n += 10
            }
        }
        n -= 4
    }
    return n
}

fn NestedLoopStale(c: bool, d: bool) uint {
    let mut n: uint := 3
    while c {
        while d {
            n += 10
        }
        n -= 4
    }
    return n
}
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatal(parsed.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	lines := []int{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OperatorIntegerOverflow {
			lines = append(lines, warning.Line)
		}
	}
	if len(lines) != 1 || lines[0] != 11 {
		t.Fatalf("S1023 lines = %v, want only the first-iteration overflow at 11; warnings %v", lines, analyzer.Warnings())
	}
}
