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

// Constant tracking precision: an overflow certain on a loop's first
// iteration warns (with that qualifier) when the condition can hold with the
// pre-loop values and every earlier write in the body ran unconditionally at
// its top level; a loop whose condition is false on entry, a later iteration
// or a write inside a nested branch proves nothing. Two branches that end
// with the same constant keep it after the join.
//
// Rules:
//   - rules/foundations/operators.md — "Compile-time overflow"
//   - rules/control-flow/flowcontrol_if.md — joins
func TestFirstIterationOverflowAndBranchConstantJoin(t *testing.T) {
	const source = `module main

fn First(c: bool) uint {
    let mut n: uint := 3
    while c {
        n -= 4
    }
    return n
}

fn NotFirst(c: bool) uint {
    let mut n: uint := 3
    while c {
        n += 10
        n -= 4
    }
    return n
}

fn Conditional(c: bool, d: bool) uint {
    let mut n: uint := 3
    while c {
        if d {
            n += 10
        }
        n -= 4
    }
    return n
}

fn NeverRuns() uint {
    let mut n: uint := 0
    while n > 0 {
        n -= 1
    }
    return n
}

fn Join(c: bool) uint {
    let mut n: uint := 1
    if c {
        n = 0
    } else {
        n = 0
    }
    n -= 1
    return n
}

fn JoinDiffer(c: bool) uint {
    let mut n: uint := 1
    if c {
        n = 0
    } else {
        n = 2
    }
    n -= 1
    return n
}

fn ForFirst(values: int[]) uint {
    let mut n: uint := 0
    for v in values {
        n -= 1
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
	got := map[int]string{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OperatorIntegerOverflow {
			got[warning.Line] = warning.Message
		}
	}
	want := map[int]bool{6: true, 46: false, 64: true}
	if len(got) != len(want) {
		t.Fatalf("S1023 warnings %v, want lines 6, 46 and 64", got)
	}
	for line, firstIteration := range want {
		message, ok := got[line]
		if !ok || strings.Contains(message, "on the first loop iteration") != firstIteration {
			t.Errorf("line %d: %q (first-iteration qualifier expected: %v)", line, message, firstIteration)
		}
	}
}

// match arms and switch clauses join like if/else: a constant every
// continuing arm or clause ends with survives (a non-exhaustive switch keeps
// the pre-switch value on its no-match path), and so does a variant proof.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 5 (MD-042)
//   - rules/control-flow/flowcontrol_switch.md — case joins
func TestMatchAndSwitchJoinConstantsAndVariants(t *testing.T) {
	const source = `module main

type Choice union {
    Yes(int)
    No
}

fn MatchSame(c: Choice) uint {
    let mut n: uint := 1
    match c {
        Yes(v) => {
            n = 0
        }
        No => {
            n = 0
        }
    }
    n -= 1
    return n
}

fn MatchDiffer(c: Choice) uint {
    let mut n: uint := 1
    match c {
        Yes(v) => {
            n = 0
        }
        No => {
            n = 2
        }
    }
    n -= 1
    return n
}

fn SwitchSame(k: int) uint {
    let mut n: uint := 1
    switch k {
    case 1:
        n = 0
    default:
        n = 0
    }
    n -= 1
    return n
}

fn SwitchPartial(k: int) uint {
    let mut n: uint := 1
    switch k {
    case 1:
        n = 0
    case 2:
        n = 0
    }
    n -= 1
    return n
}

fn MatchVariant(c: Choice, o: Option[int]) int {
    let mut x := o
    match c {
        Yes(v) => {
            x = Some(v)
        }
        No => {
            x = Some(0)
        }
    }
    match x {
        Some(v) => {
            return v
        }
    }
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
	lines := map[int]bool{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OperatorIntegerOverflow {
			lines[warning.Line] = true
		}
	}
	if len(lines) != 2 || !lines[18] || !lines[44] {
		t.Fatalf("S1023 lines = %v, want 18 (match) and 44 (switch)", lines)
	}
}

// Every sibling branch of a join starts from the constants before the join:
// an else branch, a later match arm and a later switch clause see the
// pre-join value even when an earlier sibling changed it, while a clause
// entered by fallthrough keeps the changed (unknown) state.
//
// Rules:
//   - rules/foundations/operators.md — "Compile-time overflow"
//   - rules/control-flow/flowcontrol_switch.md — fallthrough
func TestSiblingBranchesStartFromPreJoinConstants(t *testing.T) {
	const source = `module main

fn ElseSees(c: bool) uint {
    let mut n: uint := 0
    if c {
        n = 5
    } else {
        n -= 1
    }
    return n
}

fn AfterJoin(c: bool) uint {
    let mut n: uint := 0
    if c {
        n = 5
    } else {
        n = 7
    }
    n -= 1
    return n
}

fn Unchanged(c: bool) uint {
    let mut n: uint := 0
    if c {
        let x := 1
    } else {
        let y := 2
    }
    n -= 1
    return n
}

type Choice union {
    Yes(int)
    No
}

fn ArmSees(c: Choice) uint {
    let mut n: uint := 0
    match c {
        Yes(v) => {
            n = 5
        }
        No => {
            n -= 1
        }
    }
    return n
}

fn ClauseSees(k: int) uint {
    let mut n: uint := 0
    switch k {
    case 1:
        n = 5
    default:
        n -= 1
    }
    return n
}

fn Fallthrough(k: int) uint {
    let mut n: uint := 0
    switch k {
    case 1:
        n = 5
        fallthrough
    case 2:
        n -= 1
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
	lines := map[int]bool{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OperatorIntegerOverflow {
			lines[warning.Line] = true
		}
	}
	if len(lines) != 4 || !lines[8] || !lines[31] || !lines[47] || !lines[59] {
		t.Fatalf("S1023 lines = %v, want 8 (else), 31 (unchanged join), 47 (match arm) and 59 (switch clause)", lines)
	}
}
