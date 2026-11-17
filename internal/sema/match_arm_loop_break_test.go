package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// match is not a break target: a break in a match arm leaves the enclosing
// loop, so code after an infinite for or while true whose only break is in a
// match arm is reachable.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 34(1)–(2)
//   - rules/control-flow/flowcontrol_for.md — nearest-loop break ownership
//   - rules/control-flow/flowcontrol_while.md — § 20
func TestBreakInMatchArmLeavesLoop(t *testing.T) {
	source := `module main

fn ForLoop(value: Option[int]) int {
    for {
        match value {
            Some(x) => {
                discard x
            }
            None => {
                break
            }
        }
    }
    return 1
}

fn WhileLoop(value: Option[int]) int {
    while true {
        match value {
            Some(x) => {
                discard x
            }
            None => {
                break
            }
        }
    }
    return 2
}
`
	parsed := parser.New(lexer.NewWithFile(source, "break.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	if errors := NewAnalyzer().Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("a break in a match arm did not leave its loop: %v", errors)
	}
}
