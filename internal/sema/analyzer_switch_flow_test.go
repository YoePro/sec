package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Closed-enum coverage and explicit fallthrough are recorded once during Sema
// and then consumed by function-return analysis without rebuilding coverage
// from enum member spellings.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §16 "Explicit fallthrough"
//   - rules/control-flow/flowcontrol_switch.md — §24 "Enum subjects"
//   - rules/control-flow/flowcontrol_switch.md — §32 "Switch termination"
//   - rules/corrections/applied/correction24-20260823.md — "Architectural correction"
func TestResolvedSwitchFlowRetainsCoverageAndFallthrough(t *testing.T) {
	input := `
module main

enum Direction { North, South }

fn Classify(direction: Direction) int {
    switch direction {
    case Direction.North:
        fallthrough
    case Direction.South:
        return 1
    }
}

fn Partial(value: int) void {
    switch value {
    case 1:
        return
    }
}
`

	parsed := parser.New(lexer.New(input))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parsed.Errors())
	}
	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(program)
	assertSemaErrors(t, errors, nil)
	switches := []*ast.SwitchStatement{}
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Body == nil {
			continue
		}
		for _, bodyStatement := range function.Body.Statements {
			if switchStatement, ok := bodyStatement.(*ast.SwitchStatement); ok {
				switches = append(switches, switchStatement)
			}
		}
	}
	if len(switches) != 2 {
		t.Fatalf("switch count = %d, want 2", len(switches))
	}

	closed, ok := analyzer.ResolvedSwitchFlowOf(switches[0])
	if !ok || !closed.HasSubject || closed.SubjectType.Name != "Direction" || !closed.Exhaustive || closed.HasUnmatchedPath {
		t.Fatalf("closed-enum switch flow = %+v, found=%v", closed, ok)
	}
	if len(closed.Clauses) != 2 || !closed.Clauses[0].FallsThrough || closed.Clauses[0].Continues || closed.Clauses[1].FallsThrough {
		t.Fatalf("closed-enum clause flow = %+v", closed.Clauses)
	}

	partial, ok := analyzer.ResolvedSwitchFlowOf(switches[1])
	if !ok || partial.Exhaustive || !partial.HasUnmatchedPath || partial.SubjectType.Kind != IntType {
		t.Fatalf("partial switch flow = %+v, found=%v", partial, ok)
	}

	closed.Clauses[0].FallsThrough = false
	again, _ := analyzer.ResolvedSwitchFlowOf(switches[0])
	if !again.Clauses[0].FallsThrough {
		t.Fatal("ResolvedSwitchFlowOf exposed mutable clause storage")
	}
	before := len(analyzer.resolvedSwitchFlows)
	if _, found := analyzer.ResolvedSwitchFlowOf(&ast.SwitchStatement{}); found || len(analyzer.resolvedSwitchFlows) != before {
		t.Fatal("unknown switch lookup mutated resolved facts")
	}
}

// Lambda return validation consumes the same resolved switch flow as ordinary
// function return validation, including semantic closed-enum coverage.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §24 "Enum subjects"
//   - rules/control-flow/flowcontrol_switch.md — §32 "Switch termination"
//   - rules/declarations/lambda-functions.md — §9 "Lambda bodies"
func TestLambdaReturnUsesResolvedSwitchFlow(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

enum Direction { North, South }

fn Build() void {
    let classify := fn(direction: Direction) int {
        switch direction {
        case Direction.North:
            return 1
        case Direction.South:
            return 2
        }
    }
    discard classify
}
`)
	assertSemaErrors(t, errors, nil)
}
