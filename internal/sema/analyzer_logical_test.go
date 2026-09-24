package sema

import (
	"testing"

	"sec/internal/ast"
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
