package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestResolvedLogicalFlowRecordsSelectedRHSEdge(t *testing.T) {
	const source = `module main

fn Evaluate(left: bool, right: bool) void {
	let conditional := left && right
	let skippedAnd := false && right
	let skippedOr := true || right
	let required := true && right
}
`
	p := parser.New(lexer.NewWithFile(source, "logical-flow.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	expressions := make([]*ast.InfixExpression, 0, len(function.Body.Statements))
	for _, statement := range function.Body.Statements {
		expressions = append(expressions, statement.(*ast.LetStatement).Value.(*ast.InfixExpression))
	}
	want := []LogicalRHSExecution{
		LogicalRHSConditional,
		LogicalRHSNever,
		LogicalRHSNever,
		LogicalRHSAlways,
	}
	for index, expression := range expressions {
		fact, ok := analyzer.ResolvedLogicalFlowOf(expression)
		if !ok {
			t.Fatalf("expression %d has no resolved logical flow", index)
		}
		if fact.RHSExecution != want[index] {
			t.Fatalf("expression %d RHS execution = %q, want %q", index, fact.RHSExecution, want[index])
		}
		if fact.Operator != expression.Operator || fact.LeftType.Kind != BoolType || fact.RightType.Kind != BoolType || fact.ResultType.Kind != BoolType {
			t.Fatalf("expression %d fact = %#v", index, fact)
		}
		if fact.EvaluateRHSWhenLeft != (expression.Operator == "&&") || fact.ShortCircuitResult != (expression.Operator == "||") {
			t.Fatalf("expression %d selected edge = %#v", index, fact)
		}
	}

	before := len(analyzer.resolvedLogicalFlows)
	if _, ok := analyzer.ResolvedLogicalFlowOf(&ast.InfixExpression{}); ok || len(analyzer.resolvedLogicalFlows) != before {
		t.Fatal("unknown logical-flow query inferred or inserted a fact")
	}
}

// The RHS of && executes only where the LHS is true. A dominating positive
// uint comparison therefore makes subtraction by one valid on that edge.
//
// Rules:
//   - rules/foundations/operators.md — "Short-circuit evaluation"
//   - rules/foundations/operators.md — "Integer arithmetic"
func TestLogicalAndPositiveUintGuardSuppressesFalseUnderflow(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn HasPrevious() bool {
	let mut index: uint := 0
	while index < 4 {
		if index > 0 && index - 1 == 0 {
			return true
		}
		index += 1
	}
	return false
}
`)
	assertSemaErrors(t, errors, nil)
}

func TestLogicalIntegerRefinementRequiresASelectedSufficientGuard(t *testing.T) {
	tests := []struct {
		name  string
		guard string
	}{
		{name: "lower bound too small", guard: "index > 0 && index - 2 == 0"},
		{name: "or selects false edge", guard: "index > 0 || index - 1 == 0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Check() bool {
	let index: uint := 0
	return `+test.guard+`
}
`)
			assertSemaErrors(t, errors, nil)
			warnings := analyzer.Warnings()
			if len(warnings) != 1 || warnings[0].ID != diagnostics.OperatorIntegerOverflow || warnings[0].Severity != diagnostics.SeverityWarning {
				t.Fatalf("warnings = %+v, want one %s warning", warnings, diagnostics.OperatorIntegerOverflow)
			}
		})
	}
}

func TestLogicalAndReversedPositiveUintGuardSuppressesFalseUnderflow(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Check() bool {
	let index: uint := 0
	return 0 < index && index - 1 == 0
}
`)
	assertSemaErrors(t, errors, nil)
}

func TestLogicalSelectedFalseEdgeRefinesUintSubtraction(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn CheckPrevious(index: uint) bool {
	return index < 1 || index - 1 == 0
}

fn CheckTwoPrevious(index: uint) bool {
	return index > 0 && (index == 1 || index - 2 == 0)
}
`)
	assertSemaErrors(t, errors, nil)
}
