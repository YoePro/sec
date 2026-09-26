package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// If path selection and continuation are resolved once during Sema, including
// the implicit false path of an if without else. Return validation consumes
// that same fact, so a statically impossible false path cannot manufacture a
// missing-return error.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §18 "Definite assignment"
//   - rules/control-flow/flowcontrol_if.md — §19 "Terminating branches"
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements"
func TestResolvedIfFlowRetainsPathExecutionAndTermination(t *testing.T) {
	input := `
module main

fn Always() int {
    if true {
        return 1
    }
}

fn FalseElse() int {
    if false {
    } else {
        return 2
    }
}

fn Conditional(flag: bool) int {
    if flag {
        return 3
    } else {
        return 4
    }
}

fn Partial(flag: bool) void {
    if flag {
        return
    }
}

fn BuildLambda() void {
    let value := fn() int {
        if true {
            return 5
        }
    }
    discard value
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

	ifs := []*ast.IfStatement{}
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Body == nil {
			continue
		}
		for _, bodyStatement := range function.Body.Statements {
			if ifStatement, ok := bodyStatement.(*ast.IfStatement); ok {
				ifs = append(ifs, ifStatement)
			}
		}
	}
	if len(ifs) != 4 {
		t.Fatalf("if count = %d, want 4", len(ifs))
	}

	always, ok := analyzer.ResolvedIfFlowOf(ifs[0])
	if !ok || always.TruePathExecution != ResolvedIfPathAlways || always.FalsePathExecution != ResolvedIfPathNever || always.HasExplicitElse || always.HasNoBranchPath || always.TruePathContinues || always.FalsePathContinues {
		t.Fatalf("literal-true if flow = %+v, found=%v", always, ok)
	}

	falseElse, ok := analyzer.ResolvedIfFlowOf(ifs[1])
	if !ok || falseElse.TruePathExecution != ResolvedIfPathNever || falseElse.FalsePathExecution != ResolvedIfPathAlways || !falseElse.HasExplicitElse || falseElse.HasNoBranchPath || falseElse.TruePathContinues || falseElse.FalsePathContinues {
		t.Fatalf("literal-false if flow = %+v, found=%v", falseElse, ok)
	}

	conditional, ok := analyzer.ResolvedIfFlowOf(ifs[2])
	if !ok || conditional.TruePathExecution != ResolvedIfPathConditional || conditional.FalsePathExecution != ResolvedIfPathConditional || !conditional.HasExplicitElse || conditional.HasNoBranchPath || conditional.TruePathContinues || conditional.FalsePathContinues {
		t.Fatalf("conditional if flow = %+v, found=%v", conditional, ok)
	}

	partial, ok := analyzer.ResolvedIfFlowOf(ifs[3])
	if !ok || partial.TruePathExecution != ResolvedIfPathConditional || partial.FalsePathExecution != ResolvedIfPathConditional || partial.HasExplicitElse || !partial.HasNoBranchPath || partial.TruePathContinues || !partial.FalsePathContinues {
		t.Fatalf("partial if flow = %+v, found=%v", partial, ok)
	}

	before := len(analyzer.resolvedIfFlows)
	if _, found := analyzer.ResolvedIfFlowOf(&ast.IfStatement{}); found || len(analyzer.resolvedIfFlows) != before {
		t.Fatal("unknown if lookup mutated resolved facts")
	}
}
